import { EventType, EventData } from './TurboEvent';

export interface NodeData extends Record<string, unknown> {
  id: string;
  status: EventType;
  dependencies: string[];
  provider: string;
  tokens: number;
  data: EventData;
}
