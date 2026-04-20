package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"tars/internal/modules/sshcredentials"
)

const encryptedSecretAlgorithm = "AES-256-GCM"

type EncryptedSecretVault struct {
	db    *sql.DB
	key   []byte
	keyID string
}

type sealedSecret struct {
	Nonce      []byte
	Ciphertext []byte
	Algorithm  string
}

func NewEncryptedSecretVault(db *sql.DB, masterKey string, keyID string) (*EncryptedSecretVault, error) {
	if db == nil {
		return nil, nil
	}
	key, err := normalizeSecretCustodyMasterKey(masterKey)
	if err != nil {
		return nil, err
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		keyID = "local"
	}
	return &EncryptedSecretVault{db: db, key: key, keyID: keyID}, nil
}

func (v *EncryptedSecretVault) Put(ctx context.Context, ref string, value []byte, metadata map[string]string) error {
	if v == nil || v.db == nil {
		return sshcredentials.ErrNotConfigured
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("secret ref is required")
	}
	sealed, err := sealSecret(v.key, value)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `
		INSERT INTO encrypted_secrets (ref, ciphertext, nonce, key_id, algorithm, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (ref) DO UPDATE SET
			ciphertext = EXCLUDED.ciphertext,
			nonce = EXCLUDED.nonce,
			key_id = EXCLUDED.key_id,
			algorithm = EXCLUDED.algorithm,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, ref, sealed.Ciphertext, sealed.Nonce, v.keyID, sealed.Algorithm, payload)
	return err
}

func (v *EncryptedSecretVault) Get(ctx context.Context, ref string) ([]byte, error) {
	if v == nil || v.db == nil {
		return nil, sshcredentials.ErrNotConfigured
	}
	ref = strings.TrimSpace(ref)
	var sealed sealedSecret
	var keyID string
	err := v.db.QueryRowContext(ctx, `
		SELECT ciphertext, nonce, key_id, algorithm
		FROM encrypted_secrets
		WHERE ref = $1
	`, ref).Scan(&sealed.Ciphertext, &sealed.Nonce, &keyID, &sealed.Algorithm)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sshcredentials.ErrNotFound
		}
		return nil, err
	}
	return openSecret(v.key, sealed)
}

func (v *EncryptedSecretVault) Delete(ctx context.Context, ref string) error {
	if v == nil || v.db == nil {
		return sshcredentials.ErrNotConfigured
	}
	_, err := v.db.ExecContext(ctx, `DELETE FROM encrypted_secrets WHERE ref = $1`, strings.TrimSpace(ref))
	return err
}

func (v *EncryptedSecretVault) Metadata(ctx context.Context, ref string) (sshcredentials.SecretMetadata, bool, error) {
	if v == nil || v.db == nil {
		return sshcredentials.SecretMetadata{}, false, sshcredentials.ErrNotConfigured
	}
	ref = strings.TrimSpace(ref)
	var meta sshcredentials.SecretMetadata
	meta.Ref = ref
	err := v.db.QueryRowContext(ctx, `
		SELECT key_id, algorithm, updated_at
		FROM encrypted_secrets
		WHERE ref = $1
	`, ref).Scan(&meta.KeyID, &meta.Algorithm, &meta.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sshcredentials.SecretMetadata{}, false, nil
		}
		return sshcredentials.SecretMetadata{}, false, err
	}
	meta.Set = true
	return meta, true, nil
}

type SSHCredentialRepository struct {
	db *sql.DB
}

func NewSSHCredentialRepository(db *sql.DB) *SSHCredentialRepository {
	if db == nil {
		return nil
	}
	return &SSHCredentialRepository{db: db}
}

func (r *SSHCredentialRepository) List(ctx context.Context) ([]sshcredentials.Credential, error) {
	if r == nil || r.db == nil {
		return nil, sshcredentials.ErrNotConfigured
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT credential_id, display_name, owner_type, owner_id, connector_id, username, auth_type, secret_ref, passphrase_secret_ref, host_scope, status, created_by, updated_by, created_at, updated_at, last_rotated_at, expires_at
		FROM ssh_credentials
		ORDER BY credential_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sshcredentials.Credential
	for rows.Next() {
		cred, err := scanSSHCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cred)
	}
	return out, rows.Err()
}

func (r *SSHCredentialRepository) Get(ctx context.Context, credentialID string) (sshcredentials.Credential, bool, error) {
	if r == nil || r.db == nil {
		return sshcredentials.Credential{}, false, sshcredentials.ErrNotConfigured
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT credential_id, display_name, owner_type, owner_id, connector_id, username, auth_type, secret_ref, passphrase_secret_ref, host_scope, status, created_by, updated_by, created_at, updated_at, last_rotated_at, expires_at
		FROM ssh_credentials
		WHERE credential_id = $1
	`, strings.TrimSpace(credentialID))
	cred, err := scanSSHCredential(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sshcredentials.Credential{}, false, nil
		}
		return sshcredentials.Credential{}, false, err
	}
	return cred, true, nil
}

func (r *SSHCredentialRepository) Save(ctx context.Context, cred sshcredentials.Credential) error {
	if r == nil || r.db == nil {
		return sshcredentials.ErrNotConfigured
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ssh_credentials (
			credential_id, display_name, owner_type, owner_id, connector_id, username, auth_type, secret_ref, passphrase_secret_ref, host_scope, status, created_by, updated_by, created_at, updated_at, last_rotated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (credential_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			owner_type = EXCLUDED.owner_type,
			owner_id = EXCLUDED.owner_id,
			connector_id = EXCLUDED.connector_id,
			username = EXCLUDED.username,
			auth_type = EXCLUDED.auth_type,
			secret_ref = EXCLUDED.secret_ref,
			passphrase_secret_ref = EXCLUDED.passphrase_secret_ref,
			host_scope = EXCLUDED.host_scope,
			status = EXCLUDED.status,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at,
			last_rotated_at = EXCLUDED.last_rotated_at,
			expires_at = EXCLUDED.expires_at
	`, cred.CredentialID, cred.DisplayName, cred.OwnerType, cred.OwnerID, cred.ConnectorID, cred.Username, cred.AuthType, cred.SecretRef, cred.PassphraseSecretRef, cred.HostScope, cred.Status, cred.CreatedBy, cred.UpdatedBy, cred.CreatedAt, cred.UpdatedAt, cred.LastRotatedAt, nullableTimePtr(cred.ExpiresAt))
	return err
}

type sshCredentialScanner interface {
	Scan(dest ...any) error
}

func scanSSHCredential(scanner sshCredentialScanner) (sshcredentials.Credential, error) {
	var cred sshcredentials.Credential
	var expiresAt sql.NullTime
	if err := scanner.Scan(
		&cred.CredentialID,
		&cred.DisplayName,
		&cred.OwnerType,
		&cred.OwnerID,
		&cred.ConnectorID,
		&cred.Username,
		&cred.AuthType,
		&cred.SecretRef,
		&cred.PassphraseSecretRef,
		&cred.HostScope,
		&cred.Status,
		&cred.CreatedBy,
		&cred.UpdatedBy,
		&cred.CreatedAt,
		&cred.UpdatedAt,
		&cred.LastRotatedAt,
		&expiresAt,
	); err != nil {
		return sshcredentials.Credential{}, err
	}
	if expiresAt.Valid {
		cred.ExpiresAt = &expiresAt.Time
	}
	return cred, nil
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func normalizeSecretCustodyMasterKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("secret custody encryption key is required")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(value) < 32 {
		return nil, fmt.Errorf("secret custody encryption key must be at least 32 bytes or base64-encoded 32 bytes")
	}
	return []byte(value[:32]), nil
}

func sealSecret(key []byte, plaintext []byte) (sealedSecret, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return sealedSecret{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return sealedSecret{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return sealedSecret{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return sealedSecret{Nonce: nonce, Ciphertext: ciphertext, Algorithm: encryptedSecretAlgorithm}, nil
}

func openSecret(key []byte, sealed sealedSecret) ([]byte, error) {
	if sealed.Algorithm != "" && sealed.Algorithm != encryptedSecretAlgorithm {
		return nil, fmt.Errorf("unsupported secret algorithm %s", sealed.Algorithm)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, sealed.Nonce, sealed.Ciphertext, nil)
}
