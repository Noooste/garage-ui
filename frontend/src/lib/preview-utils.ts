export type PreviewKind = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'none';

export const TEXT_PREVIEW_MAX_BYTES = 5 * 1024 * 1024;
export const TEXT_HIGHLIGHT_MAX_BYTES = 1 * 1024 * 1024;
export const IMAGE_PREVIEW_MAX_BYTES = 20 * 1024 * 1024;
export const PDF_PREVIEW_MAX_BYTES = 20 * 1024 * 1024;

const imageMimeByExtension: Record<string, string> = {
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  avif: 'image/avif',
  bmp: 'image/bmp',
  ico: 'image/x-icon',
  svg: 'image/svg+xml',
};

const videoExtensions = new Set(['mp4', 'webm', 'ogv', 'mov']);
const audioExtensions = new Set(['mp3', 'wav', 'ogg', 'flac', 'm4a']);
const textExtensions = new Set([
  'json', 'yaml', 'yml', 'ini', 'toml', 'xml', 'csv', 'md', 'log', 'txt',
  'conf', 'cfg', 'env', 'sh', 'bash', 'py', 'js', 'ts', 'jsx', 'tsx',
  'go', 'rs', 'java', 'c', 'h', 'cpp', 'hpp', 'sql', 'html', 'css', 'dockerfile',
]);

function getExtension(key: string): string {
  const name = key.split('/').pop() ?? '';
  if (name.toLowerCase() === 'dockerfile') return 'dockerfile';
  const dot = name.lastIndexOf('.');
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
}

// Content-Type first, extension fallback. The fallback matters: Garage often
// stores application/octet-stream, and the backend rewrites unsafe types to
// it, so a generic type defers to the file extension.
export function getPreviewKind(contentType: string | undefined, key: string): PreviewKind {
  const ct = (contentType ?? '').split(';')[0].trim().toLowerCase();
  if (ct && ct !== 'application/octet-stream') {
    if (ct.startsWith('image/')) return 'image';
    if (ct.startsWith('video/')) return 'video';
    if (ct.startsWith('audio/')) return 'audio';
    if (ct === 'application/pdf') return 'pdf';
    if (ct.startsWith('text/') || ct === 'application/json') return 'text';
  }
  const ext = getExtension(key);
  if (ext in imageMimeByExtension) return 'image';
  if (videoExtensions.has(ext)) return 'video';
  if (audioExtensions.has(ext)) return 'audio';
  if (ext === 'pdf') return 'pdf';
  if (textExtensions.has(ext)) return 'text';
  return 'none';
}

// Byte limit per kind. null means no limit because media streams with Range
// requests and is never fully loaded into memory.
export function getPreviewSizeLimit(kind: PreviewKind): number | null {
  switch (kind) {
    case 'text':
      return TEXT_PREVIEW_MAX_BYTES;
    case 'image':
      return IMAGE_PREVIEW_MAX_BYTES;
    case 'pdf':
      return PDF_PREVIEW_MAX_BYTES;
    case 'video':
    case 'audio':
      return null;
    case 'none':
      return 0;
  }
}

// The backend rewrites unsafe Content-Types to application/octet-stream, so
// blob URLs need their MIME type restored for the SVG <img> case and the
// browser PDF viewer.
export function getPreviewMime(kind: PreviewKind, contentType: string | undefined, key: string): string {
  if (kind === 'image') {
    return imageMimeByExtension[getExtension(key)] ?? contentType ?? 'application/octet-stream';
  }
  if (kind === 'pdf') return 'application/pdf';
  return contentType || 'application/octet-stream';
}

const languageByExtension: Record<string, string> = {
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  ini: 'ini',
  toml: 'ini',
  xml: 'xml',
  md: 'markdown',
  sh: 'bash',
  bash: 'bash',
  py: 'python',
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  go: 'go',
  sql: 'sql',
  html: 'xml',
  css: 'css',
  dockerfile: 'dockerfile',
};

export function getHighlightLanguage(key: string): string | null {
  return languageByExtension[getExtension(key)] ?? null;
}

// A text preview only makes sense for content that decodes as text. More
// than 10 percent replacement or non-whitespace control characters in the
// sample means the file is binary despite its name.
export function looksBinary(sample: string): boolean {
  if (sample.length === 0) return false;
  let suspicious = 0;
  let total = 0;
  for (const ch of sample) {
    total++;
    const code = ch.codePointAt(0) ?? 0;
    if (code === 0xfffd || (code < 32 && code !== 9 && code !== 10 && code !== 13)) {
      suspicious++;
    }
  }
  return suspicious / total > 0.1;
}
