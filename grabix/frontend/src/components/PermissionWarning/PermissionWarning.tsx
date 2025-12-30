import { useEffect, useState } from 'react';
import { CheckAccessibilityPermission, RequestAccessibilityPermission } from '../../../wailsjs/go/main/App';
import './PermissionWarning.css';

interface PermissionWarningProps {
  onClose?: () => void;
}

function PermissionWarning({ onClose }: PermissionWarningProps) {
  const [hasPermission, setHasPermission] = useState(true);
  const [isChecking, setIsChecking] = useState(true);

  useEffect(() => {
    checkPermission();
  }, []);

  const checkPermission = async () => {
    try {
      setIsChecking(true);
      const result = await CheckAccessibilityPermission();
      setHasPermission(result);
    } catch (error) {
      console.error('Failed to check permission:', error);
    } finally {
      setIsChecking(false);
    }
  };

  const handleRequestPermission = async () => {
    try {
      await RequestAccessibilityPermission();
      // Wait a bit then recheck
      setTimeout(() => {
        checkPermission();
      }, 1000);
    } catch (error) {
      console.error('Failed to request permission:', error);
    }
  };

  if (isChecking) {
    return null;
  }

  if (hasPermission) {
    return null;
  }

  return (
    <div className="permission-warning-overlay">
      <div className="permission-warning">
        <div className="permission-icon">⚠️</div>
        <h2>Accessibility Permission Required</h2>
        <p>
          Grabix needs Accessibility permissions to enable global keyboard shortcuts.
        </p>
        <div className="permission-steps">
          <h3>How to enable:</h3>
          <ol>
            <li>Click "Open System Settings" below</li>
            <li>Go to <strong>Privacy & Security</strong> → <strong>Accessibility</strong></li>
            <li>Click the 🔒 lock icon and enter your password</li>
            <li>Enable the checkbox for <strong>Grabix</strong> (or your Terminal/IDE)</li>
            <li>Restart the app</li>
          </ol>
        </div>
        <div className="permission-actions">
          <button className="btn-primary" onClick={handleRequestPermission}>
            Open System Settings
          </button>
          <button className="btn-secondary" onClick={checkPermission}>
            I've Granted Permission
          </button>
          {onClose && (
            <button className="btn-text" onClick={onClose}>
              Remind Me Later
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export default PermissionWarning;

