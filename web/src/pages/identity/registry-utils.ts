export function splitCSV(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

export function joinCSV(values?: string[]): string {
  return (values || []).join(', ');
}

export function previewList(values?: string[], limit = 3): string {
  const items = (values || []).filter(Boolean);
  if (!items.length) {
    return 'none';
  }
  if (items.length <= limit) {
    return items.join(', ');
  }
  return `${items.slice(0, limit).join(', ')} +${items.length - limit}`;
}

export function parseKeyValueText(value: string): Record<string, string> {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .reduce<Record<string, string>>((result, line) => {
      const separatorIndex = line.indexOf('=');
      if (separatorIndex === -1) {
        result[line] = '';
        return result;
      }
      const key = line.slice(0, separatorIndex).trim();
      const fieldValue = line.slice(separatorIndex + 1).trim();
      if (key) {
        result[key] = fieldValue;
      }
      return result;
    }, {});
}

export function formatKeyValueText(values?: Record<string, string>): string {
  return Object.entries(values || {})
    .map(([key, value]) => `${key}=${value}`)
    .join('\n');
}
