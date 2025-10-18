import React from 'react';
import { TurboEvent } from '../types/TurboEvent';
import styles from '../styles/EventLog.module.css';

interface EventLogProps {
  events: TurboEvent[];
}

export const EventLog: React.FC<EventLogProps> = ({ events }) => {
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
