import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { objectsApi } from '@/lib/api';
import { useObjectPreview } from './useObjectPreview';

vi.mock('@/lib/api', () => ({
  objectsApi: {
    get: vi.fn(),
    getPreviewUrl: vi.fn(),
  },
}));

const mockedGet = vi.mocked(objectsApi.get);
const mockedGetPreviewUrl = vi.mocked(objectsApi.getPreviewUrl);

beforeAll(() => {
  globalThis.URL.createObjectURL = vi.fn(() => 'blob:mock-url');
  globalThis.URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  vi.clearAllMocks();
});

// One client per test, created outside the component so rerenders reuse it.
function createWrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe('useObjectPreview', () => {
  it('fetches a blob and produces an object url for images', async () => {
    mockedGet.mockResolvedValue(new Blob([new Uint8Array([1, 2, 3])]));
    const { result } = renderHook(() => useObjectPreview('b', 'pic.png', 100, 'image/png'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.kind).toBe('image');
    expect(result.current.objectUrl).toBe('blob:mock-url');
    expect(mockedGet).toHaveBeenCalledWith('b', 'pic.png');
  });

  it('decodes text content', async () => {
    mockedGet.mockResolvedValue(new Blob(['{"a": 1}']));
    const { result } = renderHook(() => useObjectPreview('b', 'data.json', 8, 'application/json'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.text).toBe('{"a": 1}');
  });

  it('reports binary content pretending to be text', async () => {
    mockedGet.mockResolvedValue(new Blob([new Uint8Array([0, 1, 2, 3, 4, 0, 1, 2, 3, 4])]));
    const { result } = renderHook(() => useObjectPreview('b', 'weird.log', 10, undefined), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('binary'));
  });

  it('does not fetch when the object is over the limit', () => {
    const { result } = renderHook(
      () => useObjectPreview('b', 'big.txt', 6 * 1024 * 1024, 'text/plain'),
      { wrapper: createWrapper() },
    );
    expect(result.current.status).toBe('too-large');
    expect(mockedGet).not.toHaveBeenCalled();
  });

  it('reports unsupported kinds without fetching', () => {
    const { result } = renderHook(() => useObjectPreview('b', 'blob.bin', 10, undefined), { wrapper: createWrapper() });
    expect(result.current.status).toBe('unsupported');
    expect(mockedGet).not.toHaveBeenCalled();
  });

  it('mints a media url for video without fetching bytes', async () => {
    mockedGetPreviewUrl.mockResolvedValue({ url: '/api/v1/buckets/b/objects/v.mp4?pt=tok', expiresAt: 'later' });
    const { result } = renderHook(() => useObjectPreview('b', 'v.mp4', 10_000_000_000, 'video/mp4'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.mediaUrl).toBe('/api/v1/buckets/b/objects/v.mp4?pt=tok');
    expect(mockedGet).not.toHaveBeenCalled();
  });

  it('re-mints once on media error, then reports error', async () => {
    mockedGetPreviewUrl.mockResolvedValue({ url: '/u?pt=1', expiresAt: 'later' });
    const { result } = renderHook(() => useObjectPreview('b', 'v.mp4', 10, 'video/mp4'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));

    mockedGetPreviewUrl.mockResolvedValue({ url: '/u?pt=2', expiresAt: 'later' });
    act(() => result.current.onMediaError());
    await waitFor(() => expect(result.current.mediaUrl).toBe('/u?pt=2'));
    expect(result.current.status).toBe('ready');

    act(() => result.current.onMediaError());
    await waitFor(() => expect(result.current.status).toBe('error'));
  });

  it('reports fetch failures and recovers on retry', async () => {
    mockedGet.mockRejectedValueOnce(new Error('network down'));
    const { result } = renderHook(() => useObjectPreview('b', 'pic.png', 100, 'image/png'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('error'));

    mockedGet.mockResolvedValue(new Blob([new Uint8Array([1])]));
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.status).toBe('ready'));
  });

  it('revokes the object url on unmount', async () => {
    mockedGet.mockResolvedValue(new Blob([new Uint8Array([1])]));
    const { result, unmount } = renderHook(() => useObjectPreview('b', 'pic.png', 100, 'image/png'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));
    unmount();
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url');
  });

  it('recovers when the target object changes after a media error', async () => {
    mockedGetPreviewUrl.mockResolvedValue({ url: '/a?pt=1', expiresAt: 'later' });
    const { result, rerender } = renderHook(
      ({ k }) => useObjectPreview('b', k, 10, 'video/mp4'),
      { wrapper: createWrapper(), initialProps: { k: 'a.mp4' } },
    );
    await waitFor(() => expect(result.current.status).toBe('ready'));

    // Exhaust the single re-mint on object A, driving it into the error state.
    act(() => result.current.onMediaError());
    await waitFor(() => expect(result.current.status).toBe('ready'));
    act(() => result.current.onMediaError());
    await waitFor(() => expect(result.current.status).toBe('error'));

    // Switch to object B on the same hook instance. The stale error from A
    // must not leave B stuck: it should load and reach ready.
    mockedGetPreviewUrl.mockResolvedValue({ url: '/b?pt=1', expiresAt: 'later' });
    rerender({ k: 'b.mp4' });
    await waitFor(() => expect(result.current.status).toBe('ready'));
    expect(result.current.mediaUrl).toBe('/b?pt=1');
  });

  it('refetches the media url on retry', async () => {
    mockedGetPreviewUrl.mockResolvedValue({ url: '/u?pt=1', expiresAt: 'later' });
    const { result } = renderHook(() => useObjectPreview('b', 'v.mp4', 10, 'video/mp4'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.status).toBe('ready'));

    mockedGetPreviewUrl.mockResolvedValue({ url: '/u?pt=2', expiresAt: 'later' });
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.mediaUrl).toBe('/u?pt=2'));
    expect(result.current.status).toBe('ready');
  });

  it('ignores a pending text decode after unmount', async () => {
    let resolveText: (value: string) => void = () => {};
    const blob = new Blob(['hello']);
    vi.spyOn(blob, 'text').mockReturnValue(new Promise<string>((res) => { resolveText = res; }));
    mockedGet.mockResolvedValue(blob);
    const { result, unmount } = renderHook(() => useObjectPreview('b', 'a.txt', 5, 'text/plain'), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.objectUrl).toBe('blob:mock-url'));

    // Unmount before the decode resolves, then resolve it. The cancelled
    // guard must swallow the late result rather than set state.
    unmount();
    act(() => resolveText('hello'));
    expect(result.current.text).toBeNull();
  });
});
