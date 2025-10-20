import React from 'react';
import { useEventsStore } from '../stores/useEventsStore';
import { ChevronsUpDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';

export const EventLog: React.FC = () => {
  const [isOpen, setIsOpen] = React.useState(false);
  const events = useEventsStore((state) => state.events);

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    const hours = date.getHours().toString().padStart(2, '0');
    const minutes = date.getMinutes().toString().padStart(2, '0');
    const seconds = date.getSeconds().toString().padStart(2, '0');
    const milliseconds = date.getMilliseconds().toString().padStart(3, '0');

    // Extract microseconds/nanoseconds from ISO string if available
    const isoString = timestamp;
    const microMatch = isoString.match(/\.(\d+)/);
    const fullFraction = microMatch ? microMatch[1] : milliseconds;

    return `${hours}:${minutes}:${seconds}.${fullFraction}`;
  };

  return (
    <Collapsible
      open={isOpen}
      onOpenChange={setIsOpen}
      className="flex w-full flex-col gap-2"
    >
      <div className="flex items-center justify-between gap-4 bg-white rounded-lg border border-gray-300 px-4 py-3">
        <h2 className="font-medium">📋 Event Log ({events.length} events)</h2>
        <CollapsibleTrigger asChild>
          <Button variant="ghost" size="icon" className="size-8">
            <ChevronsUpDown className="h-4 w-4" />
            <span className="sr-only">Toggle</span>
          </Button>
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent className="flex flex-col gap-2">
        <div className="bg-white rounded-lg border border-gray-300 p-4 max-h-[300px] overflow-y-auto">
          {events.length === 0 ? (
            <div className="text-sm text-gray-500 text-center py-4">No events yet</div>
          ) : (
            <div className="flex flex-col gap-2">
              {events.map((event, index) => (
                <div
                  key={`${event.node_id}-${index}`}
                  className="flex items-center gap-3 text-sm py-1 border-b border-gray-100 last:border-b-0"
                >
                  <span className="text-gray-500 font-mono text-xs min-w-[80px]">
                    {formatTime(event.timestamp)}
                  </span>
                  <span className="font-medium text-gray-700 min-w-[140px]">
                    {event.type}
                  </span>
                  <span className="text-gray-500 font-mono text-xs">
                    {event.node_id.substring(0, 8)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
};
