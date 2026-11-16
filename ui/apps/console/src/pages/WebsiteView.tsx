/* eslint-disable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex -- The remote screen is an application surface forwarding keyboard and pointer input, not a local document. */
import { useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, Callout } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import { useAuthStore } from "@/stores/authStore";

type Event = { action: string; x?: number; y?: number; text?: string };
type Frame = { image?: string; url?: string; message?: string };

/** Displays an isolated remote browser and forwards user input through the Hub. */
export default function WebsiteView() {
  const { websiteId = "" } = useParams();
  const userId = useAuthStore((state) => state.userId);
  const [frame, setFrame] = useState<Frame>({});
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const send = useRef<(event: Event) => void>(() => {});
  const lastMove = useRef(0);

  useEffect(() => {
    const session = crypto.randomUUID?.() ?? Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, "0")).join("");
    const path = `/assigned-websites/${encodeURIComponent(websiteId)}/browser`;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const queue: Event[] = [];
    send.current = (event) => {
      const last = queue[queue.length - 1];
      if (last?.action === event.action && event.action === "move") queue[queue.length - 1] = event;
      else if (last?.action === "wheel" && event.action === "wheel") {
        last.x = (last.x ?? 0) + (event.x ?? 0);
        last.y = (last.y ?? 0) + (event.y ?? 0);
      } else if (last?.action === "text" && event.action === "text" && ((last.text?.length ?? 0) + (event.text?.length ?? 0)) <= 4096) last.text = (last.text ?? "") + (event.text ?? "");
      else if (queue.length < 100) queue.push(event);
    };
    const request = (event: Event) => directoryRequest<Frame>(path, "POST", { session, ...event });
    const tick = async (event: Event) => {
      try {
        const result = await request(event);
        if (stopped) return;
        setFrame(result);
        setReady(true);
        setError("");
        timer = setTimeout(() => void tick(queue.shift() ?? { action: "frame" }), queue.length ? 0 : 500);
      } catch (err) {
        if (!stopped) {
          setError(err instanceof Error ? err.message : "Browser connection failed.");
          setReady(false);
        }
      } finally {
        if (stopped) void request({ action: "close" }).catch(() => {});
      }
    };
    queueMicrotask(() => {
      if (stopped) return;
      setReady(false);
      setFrame({});
      setError("");
      void tick({ action: "start" });
    });
    return () => {
      stopped = true;
      clearTimeout(timer);
      send.current = () => {};
      void request({ action: "close" }).catch(() => {});
    };
  }, [websiteId, userId, attempt]);

  return (
    <main className="flex h-dvh flex-col bg-background text-text-primary">
      <header className="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
        <Link to="/websites" className="underline">Websites</Link>
        <Button variant="secondary" size="sm" disabled={!ready} onClick={() => send.current({ action: "back" })}>Back</Button>
        <Button variant="secondary" size="sm" disabled={!ready} onClick={() => send.current({ action: "forward" })}>Forward</Button>
        <Button variant="secondary" size="sm" disabled={!ready} onClick={() => send.current({ action: "reload" })}>Reload</Button>
        <span className="min-w-0 flex-1 truncate text-sm" title={frame.url}>{frame.url || "Remote browser"}</span>
        <span className="text-sm text-text-secondary">Browsing through Hub · Downloads disabled</span>
      </header>
      {!ready && !error && <p role="status" className="p-4">Starting your browser session…</p>}
      {error && <div className="p-4"><Callout variant="error">{error}</Callout><Button onClick={() => setAttempt((value) => value + 1)}>Reconnect</Button></div>}
      {frame.message && <p role="status" className="px-4 py-2">{frame.message}</p>}
      {frame.image && (
        <div className="min-h-0 flex-1 overflow-auto bg-neutral-900">
        <img
          src={frame.image}
          alt="Interactive remote website. Click to focus, then type or scroll."
          draggable={false}
          tabIndex={0}
          role="application"
          aria-label="Remote website browser"
          className="mx-auto block w-full max-w-[1280px] outline-none focus:ring-2 focus:ring-blue-500"
          onContextMenu={(event) => event.preventDefault()}
          onClick={(event) => {
            event.currentTarget.focus();
            const bounds = event.currentTarget.getBoundingClientRect();
            send.current({ action: "click", x: (event.clientX - bounds.left) * 1280 / bounds.width, y: (event.clientY - bounds.top) * 800 / bounds.height });
          }}
          onMouseMove={(event) => {
            if (Date.now() - lastMove.current < 200) return;
            lastMove.current = Date.now();
            const bounds = event.currentTarget.getBoundingClientRect();
            send.current({ action: "move", x: (event.clientX - bounds.left) * 1280 / bounds.width, y: (event.clientY - bounds.top) * 800 / bounds.height });
          }}
          onWheel={(event) => {
            send.current({ action: "wheel", x: event.deltaX, y: event.deltaY * (event.deltaMode === 1 ? 16 : 1) });
          }}
          onPaste={(event) => {
            event.preventDefault();
            send.current({ action: "text", text: event.clipboardData.getData("text").slice(0, 4096) });
          }}
          onKeyDown={(event) => {
            if ((event.metaKey || event.ctrlKey) && event.key === "v") return;
            if (event.key === "Shift" || event.key === "Control" || event.key === "Alt" || event.key === "Meta") return;
            if (event.key.length !== 1 && !/^(Enter|Tab|Backspace|Delete|Escape|ArrowUp|ArrowDown|ArrowLeft|ArrowRight|Home|End|PageUp|PageDown)$/.test(event.key)) return;
            event.preventDefault();
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
              send.current({ action: "text", text: event.key });
            } else {
              const modifiers = `${event.ctrlKey || event.metaKey ? "Control+" : ""}${event.altKey ? "Alt+" : ""}${event.shiftKey ? "Shift+" : ""}`;
              send.current({ action: "key", text: modifiers + (event.key === " " ? "Space" : event.key) });
            }
          }}
        />
        </div>
      )}
    </main>
  );
}
