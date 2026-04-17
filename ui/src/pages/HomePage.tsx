import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

interface HomePageProps {
  datasourceConfigured: boolean | undefined;
  statusLoaded: boolean;
}

export default function HomePage({ datasourceConfigured, statusLoaded }: HomePageProps): JSX.Element | null {
  const navigate = useNavigate();

  useEffect(() => {
    if (!statusLoaded || datasourceConfigured) {
      return;
    }
    navigate('/datasource', { replace: true });
  }, [datasourceConfigured, navigate, statusLoaded]);

  if (!statusLoaded) {
    return null;
  }

  return null;
}
