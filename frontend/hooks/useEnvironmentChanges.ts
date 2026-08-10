import { useEffect, useState, useRef } from 'react';
import toast from 'react-hot-toast';

export interface EnvironmentChangeMessage {
  type: string;
  file_path: string;
  user_id: string;
  user_name: string;
  action: string;
  timestamp: string;
  diff?: string;
  commit_hash?: string;
  message?: string;
}

export function useEnvironmentChanges(envId: string | undefined) {
  const [activeEditors, setActiveEditors] = useState<string[]>([]);
  const [hasUncommittedChanges, setHasUncommittedChanges] = useState<boolean>(false);
  const [recentChanges, setRecentChanges] = useState<EnvironmentChangeMessage[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!envId) return;

    // Connect to WS
    // Note: in a real app you might proxy this or use wss:// if https.
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host || 'localhost:3000';
    
    // We append the auth token to the URL or we let the browser send cookies (if SameSite permits)
    const ws = new WebSocket(`${protocol}//${host}/api/ws/environments/${envId}`);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const msg: EnvironmentChangeMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'file_changed':
            toast.success(`${msg.user_name} modified ${msg.file_path}`);
            setHasUncommittedChanges(true);
            setRecentChanges((prev) => [msg, ...prev].slice(0, 50));
            
            setActiveEditors((prev) => {
              if (!prev.includes(msg.user_name)) {
                return [...prev, msg.user_name];
              }
              return prev;
            });
            break;

          case 'committed':
            toast.success(`${msg.user_name} committed changes: ${msg.message}`);
            setHasUncommittedChanges(false);
            setRecentChanges([]); // clear local diffs
            setActiveEditors([]); // clear active editors conceptually
            break;
            
          case 'rebuild_triggered':
            toast(`Environment rebuild triggered by ${msg.user_name}`, { icon: 'ℹ️' });
            break;
        }
      } catch (err) {
        console.error("Failed to parse websocket message", err);
      }
    };

    ws.onclose = () => {
      console.log('WS disconnected');
    };

    return () => {
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close();
      }
    };
  }, [envId]);

  return {
    hasUncommittedChanges,
    setHasUncommittedChanges, // allow manual override
    activeEditors,
    recentChanges,
  };
}
