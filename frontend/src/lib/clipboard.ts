/**
 * Copy text to the clipboard, returning whether it worked.
 *
 * navigator.clipboard only exists in a secure context, and garage-ui is usually
 * reached over plain HTTP, so the clipboard object is often missing entirely.
 * The deprecated execCommand path still works there and in every browser we
 * support, so it stays as the fallback.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // Permission denied or an unfocused document, fall through.
  }

  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  area.style.position = 'fixed';
  area.style.opacity = '0';
  document.body.appendChild(area);
  try {
    area.focus();
    area.select();
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    area.remove();
  }
}
