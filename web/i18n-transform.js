import { Project, SyntaxKind, ts } from 'ts-morph';
import * as fs from 'fs';

const project = new Project({
  tsConfigFilePath: 'tsconfig.app.json',
});

// Exclude ui components and hooks, we mainly target pages and high-level layout/components
const sourceFiles = project.getSourceFiles([
  'src/pages/**/*.tsx',
  'src/components/layout/**/*.tsx',
  'src/components/operator/**/*.tsx',
  'src/components/list/**/*.tsx',
]);

const attributesToTranslate = ['label', 'title', 'subtitle', 'description', 'placeholder', 'tooltip'];

const newTranslations = new Set();
let changedFilesCount = 0;

function escapeString(str) {
  return str.replace(/'/g, "\\'");
}

function processFile(file) {
  let changed = false;

  const jsxElements = file.getDescendantsOfKind(SyntaxKind.JsxElement);
  const jsxSelfClosingElements = file.getDescendantsOfKind(SyntaxKind.JsxSelfClosingElement);
  const jsxTextNodes = file.getDescendantsOfKind(SyntaxKind.JsxText);

  // Function to wrap text
  const wrapText = (text) => {
    const trimmed = text.trim();
    if (!trimmed || trimmed.length <= 1 || !/[a-zA-Z\u4e00-\u9fa5]/.test(trimmed)) return null;
    
    // Skip camelCase or snake_case that looks like code variables
    if (/^[a-z]+[A-Z][a-zA-Z]*$/.test(trimmed) && trimmed.length < 15) return null;
    if (/^[a-z_]+$/.test(trimmed)) return null;
    // Skip purely numbers/symbols
    if (/^[\d\s\-.,!@#$%^&*()_+={}\[\]:;"'<>/?|\\~`]+$/.test(trimmed)) return null;

    newTranslations.add(trimmed);
    return `t('${escapeString(trimmed)}', '${escapeString(trimmed)}')`;
  };

  // Process JSX Text
  for (const textNode of jsxTextNodes) {
    const parent = textNode.getParent();
    // Ignore script tags or style tags
    if (parent.getKind() === SyntaxKind.JsxElement) {
        const tagName = parent.getOpeningElement().getTagNameNode().getText();
        if (tagName === 'script' || tagName === 'style' || tagName === 'code' || tagName === 'pre') continue;
    }

    const text = textNode.getLiteralText();
    const wrapped = wrapText(text);
    if (wrapped) {
      // Replace JSX text with JSX expression
      textNode.replaceWithText(`{${wrapped}}`);
      changed = true;
    }
  }

  // Process JSX Attributes
  const allElements = [...jsxElements.map(e => e.getOpeningElement()), ...jsxSelfClosingElements];
  for (const element of allElements) {
    for (const attr of element.getAttributes()) {
      if (attr.getKind() === SyntaxKind.JsxAttribute) {
        const name = attr.getNameNode().getText();
        if (attributesToTranslate.includes(name)) {
          const init = attr.getInitializer();
          if (init && init.getKind() === SyntaxKind.StringLiteral) {
            const text = init.getLiteralText();
            const wrapped = wrapText(text);
            if (wrapped) {
              attr.setInitializer(`{${wrapped}}`);
              changed = true;
            }
          }
        }
      }
    }
  }

  if (changed) {
    // Add import if not present
    const importDecs = file.getImportDeclarations();
    const hasI18nImport = importDecs.some(imp => importDecs.some(i => i.getModuleSpecifierValue().includes('useI18n')));
    
    if (!hasI18nImport) {
        // Calculate relative path to src/hooks/useI18n
        const filePath = file.getFilePath();
        const srcIndex = filePath.indexOf('/src/');
        const relativeDepth = filePath.substring(srcIndex + 5).split('/').length - 1;
        const prefix = relativeDepth === 0 ? './' : '../'.repeat(relativeDepth);
        file.addImportDeclaration({
            namedImports: ['useI18n'],
            moduleSpecifier: `${prefix}hooks/useI18n`
        });
    }

    // Try to inject const { t } = useI18n(); into the main component
    const functions = [...file.getFunctions(), ...file.getVariableDeclarations().filter(v => v.getInitializerIfKind(SyntaxKind.ArrowFunction))];
    for (const func of functions) {
        let body;
        if (func.getKind() === SyntaxKind.FunctionDeclaration) {
            body = func.getBody();
        } else if (func.getKind() === SyntaxKind.VariableDeclaration) {
             const init = func.getInitializerIfKind(SyntaxKind.ArrowFunction);
             body = init ? init.getBody() : null;
        }

        if (body && body.getKind() === SyntaxKind.Block) {
             // check if it returns JSX
             const hasJsx = body.getDescendantsOfKind(SyntaxKind.JsxElement).length > 0 || body.getDescendantsOfKind(SyntaxKind.JsxSelfClosingElement).length > 0;
             if (hasJsx) {
                 const hasT = body.getVariableStatements().some(v => v.getText().includes('useI18n'));
                 if (!hasT) {
                     body.insertStatements(0, 'const { t } = useI18n();');
                 }
             }
        }
    }

    file.saveSync();
    changedFilesCount++;
    console.log(`Transformed: ${file.getBaseName()}`);
  }
}

for (const file of sourceFiles) {
  processFile(file);
}

console.log(`\nProcessed ${changedFilesCount} files.`);
fs.writeFileSync('extracted_strings.json', JSON.stringify([...newTranslations], null, 2));
