import React from 'react';
import { useEventsStore } from '../stores/useEventsStore';
import styles from '../styles/EventLog.module.css';

export const EventLog: React.FC = () => {
  const events = useEventsStore((state) => state.events);

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleTimeString();
  };

  return (
    <div className={styles.eventLog}>
      <h2>📋 Event Log</h2>
      <div className={styles.eventList}>
        {events.map((event, index) => (
          <div key={`${event.node_id}-${index}`} className={styles.event}>
            <div>
              <span className={styles.eventTime}>{formatTime(event.timestamp)}</span>
              <span className={styles.eventType}>{event.type}</span>
              <span className={styles.eventNode}>{event.node_id.substring(0, 8)}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
