import { useAuth } from '../../app/providers/AuthContext';
import { useNavigate } from 'react-router-dom';
import styles from './UserMenu.module.css';

export default function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;

  const letter = (user.email?.trim()?.[0] ?? "?").toUpperCase();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className={styles.menu}>
      <div className={styles.avatar} title={user.email}>
        {letter}
      </div>
      <button onClick={handleLogout} className={styles.logoutBtn}>
        Выйти
      </button>
    </div>
  );
}