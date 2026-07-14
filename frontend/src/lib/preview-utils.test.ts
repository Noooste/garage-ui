import { describe, expect, it } from 'vitest';
import {
  getHighlightLanguage,
  getPreviewKind,
  getPreviewMime,
  getPreviewSizeLimit,
  looksBinary,
  IMAGE_PREVIEW_MAX_BYTES,
  PDF_PREVIEW_MAX_BYTES,
  TEXT_PREVIEW_MAX_BYTES,
} from './preview-utils';

describe('getPreviewKind', () => {
  it('trusts a specific content type first', () => {
    expect(getPreviewKind('image/png', 'noext')).toBe('image');
    expect(getPreviewKind('video/mp4', 'noext')).toBe('video');
    expect(getPreviewKind('audio/mpeg', 'noext')).toBe('audio');
    expect(getPreviewKind('application/pdf', 'noext')).toBe('pdf');
    expect(getPreviewKind('text/plain', 'noext')).toBe('text');
    expect(getPreviewKind('application/json', 'noext')).toBe('text');
  });

  it('ignores content type parameters', () => {
    expect(getPreviewKind('text/plain; charset=utf-8', 'noext')).toBe('text');
  });

  it('falls back to the extension when the type is octet-stream', () => {
    expect(getPreviewKind('application/octet-stream', 'photo.JPG')).toBe('image');
    expect(getPreviewKind('application/octet-stream', 'clip.mp4')).toBe('video');
    expect(getPreviewKind('application/octet-stream', 'song.flac')).toBe('audio');
    expect(getPreviewKind('application/octet-stream', 'doc.pdf')).toBe('pdf');
    expect(getPreviewKind('application/octet-stream', 'conf.yaml')).toBe('text');
    expect(getPreviewKind(undefined, 'a/b/c.json')).toBe('text');
  });

  it('detects svg as image even though the backend rewrites its type', () => {
    expect(getPreviewKind('application/octet-stream', 'logo.svg')).toBe('image');
  });

  it('detects Dockerfile without an extension', () => {
    expect(getPreviewKind(undefined, 'build/Dockerfile')).toBe('text');
  });

  it('returns none for unknown files', () => {
    expect(getPreviewKind('application/octet-stream', 'blob.bin')).toBe('none');
    expect(getPreviewKind(undefined, 'noext')).toBe('none');
    expect(getPreviewKind(undefined, 'archive.zip')).toBe('none');
  });

  it('does not treat a dotfile name as an extension', () => {
    expect(getPreviewKind(undefined, '.gitignore')).toBe('none');
  });
});

describe('getPreviewSizeLimit', () => {
  it('caps documents per kind and leaves media unlimited', () => {
    expect(getPreviewSizeLimit('text')).toBe(TEXT_PREVIEW_MAX_BYTES);
    expect(getPreviewSizeLimit('image')).toBe(IMAGE_PREVIEW_MAX_BYTES);
    expect(getPreviewSizeLimit('pdf')).toBe(PDF_PREVIEW_MAX_BYTES);
    expect(getPreviewSizeLimit('video')).toBeNull();
    expect(getPreviewSizeLimit('audio')).toBeNull();
    expect(getPreviewSizeLimit('none')).toBe(0);
  });
});

describe('getPreviewMime', () => {
  it('restores the mime type the backend rewrote', () => {
    expect(getPreviewMime('image', 'application/octet-stream', 'logo.svg')).toBe('image/svg+xml');
    expect(getPreviewMime('pdf', 'application/octet-stream', 'doc.pdf')).toBe('application/pdf');
  });
  it('keeps a usable content type when there is no mapping', () => {
    expect(getPreviewMime('text', 'text/plain', 'readme.txt')).toBe('text/plain');
    expect(getPreviewMime('text', undefined, 'readme.weird')).toBe('application/octet-stream');
  });
});

describe('getHighlightLanguage', () => {
  it('maps common extensions', () => {
    expect(getHighlightLanguage('a.json')).toBe('json');
    expect(getHighlightLanguage('a.yml')).toBe('yaml');
    expect(getHighlightLanguage('a.tsx')).toBe('typescript');
    expect(getHighlightLanguage('Dockerfile')).toBe('dockerfile');
  });
  it('returns null for unmapped extensions', () => {
    expect(getHighlightLanguage('a.log')).toBeNull();
  });
});

describe('looksBinary', () => {
  it('accepts ordinary text with newlines and tabs', () => {
    expect(looksBinary('hello\n\tworld\r\n')).toBe(false);
    expect(looksBinary('')).toBe(false);
  });
  it('flags replacement-heavy content', () => {
    expect(looksBinary('���ab')).toBe(true);
  });
  it('flags control-character-heavy content', () => {
    expect(looksBinary('\x00\x01\x02abc')).toBe(true);
  });
  it('tolerates a small fraction of oddities', () => {
    expect(looksBinary('a'.repeat(99) + '\x00')).toBe(false);
  });
});
