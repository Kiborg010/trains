import { useState } from "react";
import EditorLayout from "../../../widgets/editor-layout/ui/EditorLayout.jsx";
import UserMenu from "../../../shared/ui/UserMenu.jsx";
import styles from "./EditorPage.module.css";

const PANELS = [
  { id: "maneuvers", label: "Расстановка" },
  { id: "coupling", label: "Сцепка" },
  { id: "movement", label: "Движение" },
  { id: "scenario", label: "Сценарий" },
  { id: "metrics", label: "Подсчёт" },
];

export default function EditorPage() {
  const [activePanel, setActivePanel] = useState("maneuvers");

  return (
    <div className={styles.page}>
      <header className={styles.topbar}>
        <div className={styles.brand}>
          <div className={styles.title}>Trains Lab</div>
          <div className={styles.subtitle}>Локомотивные маневры</div>
        </div>
        <UserMenu />
      </header>

      <div className={styles.subbar}>
        <div className={styles.tabs}>
          {PANELS.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => setActivePanel(p.id)}
              className={`${styles.tab} ${activePanel === p.id ? styles.tabActive : ""}`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      <main className={styles.content}>
        <EditorLayout activePanel={activePanel} setActivePanel={setActivePanel} />
      </main>
    </div>
  );
}