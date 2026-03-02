import { useAuth } from '../../app/providers/AuthContext';
import { useNavigate } from 'react-router-dom';
import styles from './UserMenu.module.css';

export default function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  if (!user) {
    return null;
  }

  return (
    <div className={styles.menu}>
      <div className={styles.userInfo}>
        <span className={styles.email}>{user.email}</span>
      </div>
      <button onClick={handleLogout} className={styles.logoutBtn}>
        Logout
      </button>
    </div>
  );
}
