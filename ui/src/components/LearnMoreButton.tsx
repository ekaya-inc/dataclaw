import { ChevronUp, Sparkles } from 'lucide-react';

import { Button } from './ui/Button';

export function LearnMoreButton({
  open,
  onToggle,
  panelId,
}: {
  open: boolean;
  onToggle: () => void;
  panelId: string;
}): JSX.Element {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      aria-expanded={open}
      aria-controls={panelId}
      onClick={onToggle}
      className="border-violet-300 bg-violet-50 text-violet-700 hover:bg-violet-100 hover:text-violet-800"
    >
      {open ? <ChevronUp className="h-4 w-4" /> : <Sparkles className="h-4 w-4 text-violet-500" />}
      Learn more
    </Button>
  );
}
