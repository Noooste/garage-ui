import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { useAuthStore } from '@/store/auth-store';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LogIn } from 'lucide-react';
import type { AuthConfig } from '@/types/auth';

interface BasicLoginFormProps {
  showOIDC?: boolean;
  config?: AuthConfig | null;
}

export function BasicLoginForm({ showOIDC = false, config }: BasicLoginFormProps) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { loginAdmin, loginOIDC } = useAuthStore();

  const returnUrl = searchParams.get('returnUrl') || '/';
  const providerName = config?.oidc?.provider || 'OIDC Provider';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      await loginAdmin(username, password);
      // Navigate to return URL on success
      navigate(decodeURIComponent(returnUrl));
    } catch (error) {
      // Error is already handled by the store and toast
      console.error('Login failed:', error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Card className="w-full">
      <CardHeader className="space-y-1">
        <div className="flex items-center justify-center mb-4">
          <img
            src="/garage.png"
            alt="Garage Logo"
            className="h-16 w-16 object-contain"
          />
        </div>
        <CardTitle className="text-2xl text-center">
          Welcome to Garage UI
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="username" className="text-sm font-medium">Username</label>
            <Input
              id="username"
              type="text"
              placeholder="Enter your username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              disabled={isLoading}
              autoComplete="username"
            />
          </div>
          <div className="space-y-2">
            <label htmlFor="password" className="text-sm font-medium">Password</label>
            <Input
              id="password"
              type="password"
              placeholder="Enter your password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              disabled={isLoading}
              autoComplete="current-password"
            />
          </div>
          <Button
            type="submit"
            className="w-full"
            disabled={isLoading || !username || !password}
          >
            {isLoading ? 'Signing in...' : 'Sign in'}
          </Button>
        </form>

        {showOIDC && (
          <div className="mt-4">
            <div className="relative mb-4">
              <div className="relative flex justify-center text-xs">
                <span className="bg-card px-2 text-muted-foreground">or</span>
              </div>
            </div>
            <Button
              type="button"
              variant="secondary"
              className="w-full"
              onClick={loginOIDC}
            >
              <LogIn className="mr-2 h-4 w-4" />
              Sign in with {providerName}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
