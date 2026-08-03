import { useState } from 'react';
import { Check, Copy, Eye, EyeOff, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { copyText } from '@/lib/clipboard';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';

export function CredentialField({
  label,
  value,
  mono = true,
  breakAll = false,
  maskable = false,
  loading = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
  breakAll?: boolean;
  maskable?: boolean;
  loading?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(!maskable);
  const copy = async () => {
    if (!value) return;
    if (!(await copyText(value))) {
      toast.error(`Could not copy ${label}. Select it and copy by hand.`);
      return;
    }
    setCopied(true);
    toast.success(`${label} copied`);
    setTimeout(() => setCopied(false), 1600);
  };
  // Copying focuses a textarea on the fallback path, which would clear whatever
  // the user just highlighted, so a click that ends a selection copies nothing.
  const copyUnlessSelecting = () => {
    if (!window.getSelection()?.toString()) copy();
  };
  const display = loading ? '' : revealed || !maskable ? value : '•'.repeat(Math.min(40, value.length || 40));
  const copyable = !loading && !!value;
  return (
    <div className="space-y-1.5">
      <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
        {label}
      </label>
      <div className="flex items-stretch gap-2">
        {/* A div, not a button: browsers set user-select: none on buttons, which
            left the value impossible to highlight when copying failed. Keyboard
            users copy through the button next to it. */}
        <div
          onClick={copyable ? copyUnlessSelecting : undefined}
          title={copyable ? 'Click to copy' : undefined}
          className={cn(
            'flex-1 min-w-0 rounded-md border border-[var(--border)] bg-[var(--surface-sunken)]',
            'px-3 py-2 text-[13.5px] transition-colors',
            copyable ? 'cursor-pointer hover:bg-[var(--accent)]' : 'opacity-70',
            mono && 'font-mono',
            breakAll ? 'break-all' : 'truncate',
          )}
        >
          {loading ? (
            <span className="inline-flex items-center gap-2 text-[var(--muted-foreground)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Loading…
            </span>
          ) : (
            display
          )}
        </div>
        {maskable && (
          <Button
            variant="secondary"
            size="icon"
            onClick={() => setRevealed((r) => !r)}
            aria-label={revealed ? 'Hide' : 'Reveal'}
            disabled={loading || !value}
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        )}
        <Button variant="secondary" size="icon" onClick={copy} aria-label={`Copy ${label}`} disabled={loading || !value}>
          {copied ? <Check className="h-4 w-4 text-[var(--primary)]" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}
