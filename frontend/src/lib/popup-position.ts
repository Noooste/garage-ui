export interface Placement {
  /** Distance from the viewport top, when the popup opens downwards. */
  top?: number;
  /** Distance from the viewport bottom, when the popup flips above its trigger. */
  bottom?: number;
  maxHeight: number;
}

/**
 * Places a floating layer against its trigger in viewport coordinates, for use
 * with `position: fixed` so no ancestor's overflow can clip it. Flips above the
 * trigger when the room below is too tight to be usable.
 */
export function placeUnder(
  rect: { top: number; bottom: number },
  viewportHeight: number,
  gap = 4,
  maxHeight = 240,
): Placement {
  const spaceBelow = viewportHeight - rect.bottom - gap * 2;
  const spaceAbove = rect.top - gap * 2;
  const flip = spaceBelow < Math.min(maxHeight, 160) && spaceAbove > spaceBelow;

  // Floor the height so a trigger at the very edge still shows a scrollable
  // popup rather than collapsing to nothing.
  const fit = (space: number) => Math.max(120, Math.min(maxHeight, space));

  return flip
    ? { bottom: viewportHeight - rect.top + gap, maxHeight: fit(spaceAbove) }
    : { top: rect.bottom + gap, maxHeight: fit(spaceBelow) };
}
