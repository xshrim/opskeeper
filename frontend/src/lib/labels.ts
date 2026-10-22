export const parseLabels = (input: string): Record<string, string> =>
  Object.fromEntries(
    input
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => {
        const separatorIndex = item.search(/[=:]/);
        if (separatorIndex === -1) return [item.trim(), ''];
        return [
          item.slice(0, separatorIndex).trim(),
          item.slice(separatorIndex + 1).trim()
        ];
      })
      .filter(([key]) => key)
  );

export const formatLabels = (labels: Record<string, string> = {}): string =>
  Object.entries(labels)
    .map(([key, value]) => (value ? `${key}=${value}` : key))
    .join(', ');
