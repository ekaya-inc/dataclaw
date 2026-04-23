import { Heart, Star } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

import { PageHeader } from '../components/PageHeader';
import { Button } from '../components/ui/Button';
import { buttonVariants } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { useSupportDismissed } from '../hooks/useSupportDismissed';

const REPO_URL = 'https://github.com/ekaya-inc/dataclaw';

export default function SupportPage(): JSX.Element {
  const [, dismiss] = useSupportDismissed();
  const navigate = useNavigate();

  const handleDismiss = (): void => {
    dismiss();
    navigate('/');
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Support DataClaw"
        description="DataClaw is open source. If it has been useful, a GitHub star helps others discover it."
      />
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <Heart className="h-5 w-5 text-rose-500" aria-hidden="true" />
            Enjoying DataClaw?
          </CardTitle>
          <CardDescription>
            A star on GitHub takes a second and goes a long way toward getting DataClaw in front of more teams who need safer agent data access.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-3">
          <a
            href={REPO_URL}
            target="_blank"
            rel="noreferrer noopener"
            onClick={dismiss}
            className={buttonVariants({ variant: 'default', size: 'default' })}
          >
            <Star className="h-4 w-4" aria-hidden="true" />
            Star on GitHub
          </a>
          <Button variant="outline" onClick={handleDismiss}>
            Dismiss
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
