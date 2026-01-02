import { useEffect, useState } from 'react';
import {
  CheckAccessibilityPermission,
  RequestAccessibilityPermission,
  CheckScreenRecordingPermission,
  RequestScreenRecordingPermission
} from '../../../wailsjs/go/main/App';
import './PermissionWarning.css';

interface PermissionWarningProps {
  onClose?: () => void;
}

type PermissionType = 'accessibility' | 'screen-recording' | null;

function PermissionWarning({ onClose }: PermissionWarningProps) {
  const [missingPermission, setMissingPermission] = useState<PermissionType>(null);
  const [isChecking, setIsChecking] = useState(true);

  useEffect(() => {
    checkPermissions();
  }, []);

  const checkPermissions = async () => {
    try {
      setIsChecking(true);

      console.log('[PermissionWarning] Checking permissions...');

      // Add small delay to let macOS settle (especially on first launch)
      await new Promise(resolve => setTimeout(resolve, 500));

      // Check Screen Recording first (more critical for screenshot app)
      const hasScreenRecording = await CheckScreenRecordingPermission();
      console.log('[PermissionWarning] Screen Recording permission:', hasScreenRecording);

      if (!hasScreenRecording) {
        console.log('[PermissionWarning] Missing Screen Recording permission');

        // Retry once after 1 second (in case of timing issue)
        console.log('[PermissionWarning] Retrying permission check in 1s...');
        await new Promise(resolve => setTimeout(resolve, 1000));

        const hasScreenRecordingRetry = await CheckScreenRecordingPermission();
        console.log('[PermissionWarning] Screen Recording permission (retry):', hasScreenRecordingRetry);

        if (!hasScreenRecordingRetry) {
          setMissingPermission('screen-recording');
          return;
        }
      }

      // Then check Accessibility (for hotkeys) - OPTIONAL
      const hasAccessibility = await CheckAccessibilityPermission();
      console.log('[PermissionWarning] Accessibility permission:', hasAccessibility);

      if (!hasAccessibility) {
        console.log('[PermissionWarning] Missing Accessibility permission (optional - for hotkeys)');
        // Don't block app, just log warning
        // User can still use app without hotkeys
      }

      console.log('[PermissionWarning] All critical permissions granted');
      setMissingPermission(null);
    } catch (error) {
      console.error('Failed to check permissions:', error);
    } finally {
      setIsChecking(false);
    }
  };

  const handleRequestPermission = async () => {
    try {
      if (missingPermission === 'screen-recording') {
        await RequestScreenRecordingPermission();
      } else if (missingPermission === 'accessibility') {
        await RequestAccessibilityPermission();
      }

      // Wait a bit then recheck
      setTimeout(() => {
        checkPermissions();
      }, 1000);
    } catch (error) {
      console.error('Failed to request permission:', error);
    }
  };

  if (isChecking) {
    return null;
  }

  if (!missingPermission) {
    return null;
  }

  const isScreenRecording = missingPermission === 'screen-recording';

  return (
    <div className="permission-warning-overlay">
      <div className="permission-warning">
        <div className="permission-icon">⚠️</div>
        <h2>{isScreenRecording ? 'Screen Recording Permission Required' : 'Accessibility Permission Required'}</h2>
        <p>
          {isScreenRecording
            ? 'Grabix needs Screen Recording permission to capture screenshots of all windows and apps.'
            : 'Grabix needs Accessibility permission to enable global keyboard shortcuts.'
          }
        </p>
        <div className="permission-steps">
          <h3>How to enable:</h3>
          <ol>
            <li>Click "Open System Settings" below</li>
            <li>Go to <strong>Privacy & Security</strong> → <strong>{isScreenRecording ? 'Screen Recording' : 'Accessibility'}</strong></li>
            <li>Click the 🔒 lock icon and enter your password</li>
            <li>Enable the checkbox for <strong>Grabix</strong> (or your Terminal/IDE if running in dev mode)</li>
            <li>Restart the app</li>
          </ol>
        </div>
        <div className="permission-actions">
          <button className="btn-primary" onClick={handleRequestPermission}>
            Open System Settings
          </button>
          <button className="btn-secondary" onClick={checkPermissions}>
            I've Granted Permission
          </button>
          <button className="btn-secondary" onClick={() => setMissingPermission(null)}>
            Skip (I'll Fix Later)
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

