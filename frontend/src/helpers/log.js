/**
 * Formats a log string for display.
 * - Newest lines appear first
 * - If more than 100 lines, shows: 50 newest + "..." + 50 oldest
 */
export function buildLogForDisplay(value) {
  if (!value) {
    return '';
  }

  const normalized = value.replaceAll('\r\n', '\n');
  const lines = normalized.split('\n');

  // drop trailing empty line from final newline
  while (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();

  // newest first
  lines.reverse();

  if (lines.length > 100) {
    const head = lines.slice(0, 50);
    const tail = lines.slice(-50);
    return [...head, '...', ...tail].join('\n');
  }

  return lines.join('\n');
}
