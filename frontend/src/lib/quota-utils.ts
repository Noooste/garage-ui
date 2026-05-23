export type QuotaUnit = 'MB' | 'GB' | 'TB';

export const QUOTA_UNIT_BYTES: Record<QuotaUnit, number> = {
  MB: 1024 * 1024,
  GB: 1024 * 1024 * 1024,
  TB: 1024 * 1024 * 1024 * 1024,
};

// Convert a byte count to a {value, unit} pair using the largest unit that
// yields an integer. Falls back to GB if the value is 0 or doesn't divide
// evenly into any unit.
export function bytesToQuotaValue(bytes: number): { value: number; unit: QuotaUnit } {
  const units: QuotaUnit[] = ['TB', 'GB', 'MB'];
  for (const unit of units) {
    const factor = QUOTA_UNIT_BYTES[unit];
    if (bytes >= factor && bytes % factor === 0) {
      return { value: bytes / factor, unit };
    }
  }
  // Doesn't divide evenly — pick the largest unit where the value is >= 1,
  // rounded for display. The user is free to change it.
  for (const unit of units) {
    const factor = QUOTA_UNIT_BYTES[unit];
    if (bytes >= factor) {
      return { value: Math.round((bytes / factor) * 100) / 100, unit };
    }
  }
  return { value: 0, unit: 'GB' };
}

export function quotaValueToBytes(value: number, unit: QuotaUnit): number {
  return Math.round(value * QUOTA_UNIT_BYTES[unit]);
}
