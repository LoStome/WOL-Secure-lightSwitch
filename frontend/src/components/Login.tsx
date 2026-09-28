import { useEffect, useState, type FormEvent } from 'react';
import { checkSetup, login } from '../services/api';
import type { SessionUser } from '../services/types';
import { getErrorMessage } from '../utils/errorMessage';

interface LoginProps { onLoginSuccess: (user: SessionUser) => void }

const Login = ({ onLoginSuccess }: LoginProps) => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [isSetupState, setIsSetupState] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void checkSetup().then(status => { if (!cancelled) setIsSetupState(status.needs_setup); }).catch(err => console.error('Failed to check setup status', err));
    return () => { cancelled = true; };
  }, []);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = await login(email, password);
      onLoginSuccess(data.user);
    } catch (err: unknown) {
      setError(getErrorMessage(err) || 'Sign in failed. Check your details and try again.');
    } finally {
      setLoading(false);
    }
  };

  return <div className="login-layout content-wrap">
    <div className="login-intro">{isSetupState && <p className="eyebrow">FIRST-TIME SETUP</p>}
      <h1>{isSetupState ? <>Make this space<br />yours.</> : <>Your devices,<br />within reach.</>}</h1>
      <p>{isSetupState ? 'Create the first administrator account to start managing your devices.' : 'Check availability, wake a device, or send a shutdown command from one clear place.'}</p>
      {isSetupState && <div className="setup-note"><strong>Your first account is an administrator</strong><span>You can add other users after signing in.</span></div>}
    </div>
    <div className="panel login-panel"><p className="eyebrow">{isSetupState ? 'GET STARTED' : 'WELCOME BACK'}</p>
      <h2>{isSetupState ? 'Create your account' : 'Sign in'}</h2>
      <p className="panel-description">{isSetupState ? 'Set up secure access for this installation.' : 'Use your account to manage your devices.'}</p>
      {error && <div role="alert" className="form-error">{error}</div>}
      <form onSubmit={handleSubmit} className="user-form">
        <div className="field"><label htmlFor="login-email">Email</label><input id="login-email" type="email" value={email} onChange={event => setEmail(event.target.value)} required maxLength={254} placeholder="name@example.com" /></div>
        <div className="field"><label htmlFor="login-password">Password</label><input id="login-password" type="password" value={password} onChange={event => setPassword(event.target.value)} required minLength={isSetupState ? 12 : undefined} maxLength={72} placeholder={isSetupState ? 'Choose a password' : 'Enter your password'} />
          {isSetupState && <p className="field-help">Use at least 12 characters.</p>}</div>
        <button className="primary-button form-submit" type="submit" disabled={loading}>{loading ? 'Please wait…' : isSetupState ? 'Create account' : 'Sign in'}</button>
      </form>
      {!isSetupState && <p className="login-help">Having trouble? Contact your administrator.</p>}
    </div>
  </div>;
};

export default Login;
