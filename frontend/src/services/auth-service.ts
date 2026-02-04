import { AuthStartLogin, AuthHandleCallback, AuthGetCurrentUser, AuthLogout, AuthIsLoggedIn } from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

export interface User {
  id: string;
  email: string;
  name: string;
  picture: string;
  username: string;
}

class AuthService {
  /**
   * Start OAuth login flow
   * Opens browser with authorization URL and waits for callback
   */
  async login(): Promise<User> {
    try {
      // Start OAuth flow - backend starts localhost callback server
      const authURL = await AuthStartLogin();
      console.log('Opening OAuth URL:', authURL);

      // Open system browser with auth URL (not window.open which doesn't work in Wails)
      BrowserOpenURL(authURL);

      // Wait for callback to complete - backend handles the localhost callback
      // and returns user after token exchange
      const user = await AuthHandleCallback('');
      return user;
    } catch (error) {
      console.error('Failed to login:', error);
      throw error;
    }
  }

  /**
   * Handle OAuth callback (called by backend)
   */
  async handleCallback(code: string): Promise<User> {
    try {
      const user = await AuthHandleCallback(code);
      return user;
    } catch (error) {
      console.error('Failed to handle callback:', error);
      throw error;
    }
  }

  /**
   * Get current authenticated user
   */
  async getCurrentUser(): Promise<User | null> {
    try {
      const user = await AuthGetCurrentUser();
      return user;
    } catch (error) {
      console.error('Failed to get current user:', error);
      return null;
    }
  }

  /**
   * Logout current user
   */
  async logout(): Promise<void> {
    try {
      await AuthLogout();
    } catch (error) {
      console.error('Failed to logout:', error);
      throw error;
    }
  }

  /**
   * Check if user is logged in
   */
  async isLoggedIn(): Promise<boolean> {
    try {
      return await AuthIsLoggedIn();
    } catch (error) {
      console.error('Failed to check login status:', error);
      return false;
    }
  }

  /**
   * Check authentication status and return user if logged in
   */
  async checkAuth(): Promise<User | null> {
    try {
      const isLoggedIn = await this.isLoggedIn();
      if (!isLoggedIn) {
        return null;
      }
      return await this.getCurrentUser();
    } catch (error) {
      console.error('Failed to check auth:', error);
      return null;
    }
  }
}

export const authService = new AuthService();
