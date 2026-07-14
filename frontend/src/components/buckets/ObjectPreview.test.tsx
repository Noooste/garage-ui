import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ObjectPreviewState } from '@/hooks/useObjectPreview';
import { useObjectPreview } from '@/hooks/useObjectPreview';
import { TEXT_HIGHLIGHT_MAX_BYTES } from '@/lib/preview-utils';
import { ObjectPreview } from './ObjectPreview';

vi.mock('@/hooks/useObjectPreview', () => ({ useObjectPreview: vi.fn() }));

const mockedHook = vi.mocked(useObjectPreview);

function state(overrides: Partial<ObjectPreviewState>): ObjectPreviewState {
  return {
    kind: 'none',
    status: 'unsupported',
    objectUrl: null,
    text: null,
    mediaUrl: null,
    retry: vi.fn(),
    onMediaError: vi.fn(),
    ...overrides,
  };
}

function renderPreview() {
  // A .json key keeps the highlight language deterministic in the text test.
  return render(
    <ObjectPreview bucket="b" objectKey="k.json" size={100} contentType="text/plain" onDownload={vi.fn()} />,
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('ObjectPreview', () => {
  it('shows the loading state', () => {
    mockedHook.mockReturnValue(state({ kind: 'image', status: 'loading' }));
    renderPreview();
    expect(screen.getByText(/loading preview/i)).toBeInTheDocument();
  });

  it('renders an image from the object url', () => {
    mockedHook.mockReturnValue(state({ kind: 'image', status: 'ready', objectUrl: 'blob:img' }));
    renderPreview();
    expect(screen.getByRole('img')).toHaveAttribute('src', 'blob:img');
  });

  it('renders video with the media url', () => {
    mockedHook.mockReturnValue(state({ kind: 'video', status: 'ready', mediaUrl: '/u?pt=t' }));
    const { container } = renderPreview();
    const video = container.querySelector('video');
    expect(video).not.toBeNull();
    expect(video).toHaveAttribute('src', '/u?pt=t');
    expect(video).not.toHaveAttribute('autoplay');
  });

  it('renders audio with the media url', () => {
    mockedHook.mockReturnValue(state({ kind: 'audio', status: 'ready', mediaUrl: '/u?pt=t' }));
    const { container } = renderPreview();
    expect(container.querySelector('audio')).toHaveAttribute('src', '/u?pt=t');
  });

  it('renders pdf in an iframe', () => {
    mockedHook.mockReturnValue(state({ kind: 'pdf', status: 'ready', objectUrl: 'blob:pdf' }));
    renderPreview();
    expect(screen.getByTitle('k.json')).toHaveAttribute('src', 'blob:pdf');
  });

  it('renders plain text immediately and highlighted text after the import resolves', async () => {
    mockedHook.mockReturnValue(state({ kind: 'text', status: 'ready', text: '{"a": 1}' }));
    const { container } = renderPreview();
    expect(container.querySelector('pre')).toHaveTextContent('{"a": 1}');
    await waitFor(() => expect(container.querySelector('code [class*="hljs-"]')).not.toBeNull());
  });

  it('shows the too-large notice with a download action', () => {
    mockedHook.mockReturnValue(state({ kind: 'text', status: 'too-large' }));
    renderPreview();
    expect(screen.getByText(/too large to preview/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /download/i })).toBeInTheDocument();
  });

  it('shows the unsupported notice', () => {
    mockedHook.mockReturnValue(state({ kind: 'none', status: 'unsupported' }));
    renderPreview();
    expect(screen.getByText(/no preview available/i)).toBeInTheDocument();
  });

  it('shows the binary notice', () => {
    mockedHook.mockReturnValue(state({ kind: 'text', status: 'binary' }));
    renderPreview();
    expect(screen.getByText(/doesn't appear to be text/i)).toBeInTheDocument();
  });

  it('shows the error state with retry wired to the hook', () => {
    const retry = vi.fn();
    mockedHook.mockReturnValue(state({ kind: 'image', status: 'error', retry }));
    renderPreview();
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(retry).toHaveBeenCalled();
  });

  // The tests below are additional to the brief's list. They were added to
  // close coverage gaps (media error and resume handling, the highlight size
  // guard, and the defensive fallback branch) found while verifying the
  // 90 percent coverage requirement on this file.

  it('skips highlighting for text over the highlight size limit', () => {
    const bigText = 'a'.repeat(TEXT_HIGHLIGHT_MAX_BYTES + 1);
    mockedHook.mockReturnValue(state({ kind: 'text', status: 'ready', text: bigText }));
    const { container } = renderPreview();
    expect(container.querySelector('code [class*="hljs-"]')).toBeNull();
    expect(container.querySelector('code')?.textContent).toHaveLength(bigText.length);
  });

  it('unmounting before the highlight import resolves is safe', async () => {
    mockedHook.mockReturnValue(state({ kind: 'text', status: 'ready', text: '{"a": 1}' }));
    const { container, unmount } = renderPreview();
    expect(() => unmount()).not.toThrow();
    expect(container.innerHTML).toBe('');
    // Let the pending dynamic import resolve after unmount. The cancelled
    // guard means the resolved callback renders nothing back into the
    // detached container, so it stays empty and no error is thrown.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(container.innerHTML).toBe('');
  });

  it('resumes the playback position after a media error and reload', () => {
    mockedHook.mockReturnValue(state({ kind: 'video', status: 'ready', mediaUrl: '/u?pt=t' }));
    const { container } = renderPreview();
    const video = container.querySelector('video')!;

    // Loading metadata with no prior error is a no-op (resume position is 0).
    fireEvent.loadedMetadata(video);

    Object.defineProperty(video, 'currentTime', { value: 42, writable: true, configurable: true });
    fireEvent.error(video);
    Object.defineProperty(video, 'currentTime', { value: 0, writable: true, configurable: true });
    fireEvent.loadedMetadata(video);
    expect(video.currentTime).toBe(42);
  });

  it('does not restore a position when the media element reported no currentTime', () => {
    mockedHook.mockReturnValue(state({ kind: 'audio', status: 'ready', mediaUrl: '/u?pt=t' }));
    const { container } = renderPreview();
    const audio = container.querySelector('audio')!;

    // The element reports no currentTime, so the captured resume position
    // falls back to 0 via the ?? 0 guard, which keeps the restore guard
    // (resumeAtRef.current > 0) false. A later loadedmetadata must then leave
    // the position untouched, unlike the sibling resume test above.
    Object.defineProperty(audio, 'currentTime', { value: undefined, writable: true, configurable: true });
    fireEvent.error(audio);
    Object.defineProperty(audio, 'currentTime', { value: 99, writable: true, configurable: true });
    fireEvent.loadedMetadata(audio);
    expect(audio.currentTime).toBe(99);
  });

  it('falls back to the generic notice for an unhandled preview kind', () => {
    mockedHook.mockReturnValue(state({ kind: 'none', status: 'ready' }));
    renderPreview();
    expect(screen.getByText(/no preview available/i)).toBeInTheDocument();
  });
});
