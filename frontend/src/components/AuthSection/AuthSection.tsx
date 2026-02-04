import React, { useEffect, useState } from 'react';
import { useAuthStore } from '../../stores/auth-store';
import styles from './AuthSection.module.css';

export const AuthSection: React.FC = () => {
  const { user, isLoading, isAuthenticated, checkAuth, login, logout } = useAuthStore();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Check auth status on mount
    checkAuth();
  }, [checkAuth]);

  const handleLogin = async () => {
    try {
      setError(null);
      await login();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to login');
    }
  };

  const handleLogout = async () => {
    try {
      setError(null);
      await logout();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to logout');
    }
  };

  if (isLoading) {
    return (
      <div className={styles.authSection}>
        <div className={styles.loading}>Loading...</div>
      </div>
    );
  }

  return (
    <div className={styles.authSection}>
      {error && (
        <div className={styles.error}>{error}</div>
      )}

      {isAuthenticated && user ? (
        <>
          <div className={styles.userInfo}>
            {user.picture && (
              <img
                src={user.picture}
                alt={user.name}
                className={styles.avatar}
              />
            )}
            <div className={styles.userDetails}>
              <div className={styles.userName}>{user.name}</div>
              <div className={styles.userEmail}>{user.email}</div>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className={styles.logoutButton}
            disabled={isLoading}
          >
            Logout
          </button>
        </>
      ) : (
        <button
          onClick={handleLogin}
          className={styles.loginButton}
          disabled={isLoading}
        >
          Login with OAuth
        </button>
      )}
    </div>
  );
};
