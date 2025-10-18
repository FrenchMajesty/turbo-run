import { EventType, EventData } from './TurboEvent';

export interface NodeData {
  id: string;
  status: EventType;
  dependencies: string[];
  provider: string;
  tokens: number;
  data: EventData;
}
