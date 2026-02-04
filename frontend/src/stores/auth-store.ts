import { create } from 'zustand';
import { authService, User } from '../services/auth-service';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;

  // Actions
  checkAuth: () => Promise<void>;
  login: () => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User | null) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: false,
  isAuthenticated: false,

  checkAuth: async () => {
    set({ isLoading: true });
    try {
      const user = await authService.checkAuth();
      set({
        user,
        isAuthenticated: user !== null,
        isLoading: false
      });
    } catch (error) {
      console.error('Failed to check auth:', error);
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false
      });
    }
  },

  login: async () => {
    set({ isLoading: true });
    try {
      // Login opens browser and waits for callback completion
      const user = await authService.login();
      set({
        user,
        isAuthenticated: true,
        isLoading: false
      });
    } catch (error) {
      console.error('Failed to login:', error);
      set({ isLoading: false });
      throw error;
    }
  },

  logout: async () => {
    set({ isLoading: true });
    try {
      await authService.logout();
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false
      });
    } catch (error) {
      console.error('Failed to logout:', error);
      set({ isLoading: false });
      throw error;
    }
  },

  setUser: (user) => {
    set({
      user,
      isAuthenticated: user !== null
    });
  },
}));
