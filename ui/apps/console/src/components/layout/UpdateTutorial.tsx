import { useState } from "react";
import { Link } from "react-router-dom";
import { ArrowRightIcon, SparklesIcon } from "@heroicons/react/24/outline";
import { Button, IconButton } from "@cloudmatehub/design-system/primitives";
import Modal from "@/components/common/Modal";
import { useNamespaceLegacy } from "@/hooks/useNamespaceLegacy";
import { useAuthStore } from "@/stores/authStore";
import { hasPermission } from "@/utils/permission";
import { latestUpdate } from "./updateTutorial";

function Tutorial({ userId }: { userId: string }) {
  const isAdmin = useAuthStore((state) => state.isAdmin);
  const role = useAuthStore((state) => state.role);
  const legacy = useNamespaceLegacy();
  const storageKey = `cloudmatehub:update-tutorial:${userId}:${latestUpdate.id}`;
  const [open, setOpen] = useState(() => {
    try {
      return localStorage.getItem(storageKey) !== "seen";
    } catch {
      return true;
    }
  });
  const [stepId, setStepId] = useState<string | null>(null);
  const steps = latestUpdate.steps.filter((step) =>
    (!step.instanceAdmin || isAdmin) &&
    (!step.permission || hasPermission(role, step.permission)) &&
    (!step.requiresAccessPolicies || legacy === false),
  );
  const index = Math.max(0, steps.findIndex((step) => step.id === stepId));
  const step = steps[index];
  const next = steps[index + 1];
  const previous = steps[index - 1];

  function close() {
    setOpen(false);
    try {
      localStorage.setItem(storageKey, "seen");
    } catch {
      // The tutorial can still be dismissed when browser storage is unavailable.
    }
  }

  if (!step) return null;

  return (
    <>
      <IconButton
        aria-label="What’s new"
        title="What’s new"
        onClick={() => {
          setStepId(null);
          setOpen(true);
        }}
      >
        <SparklesIcon className="h-4 w-4" />
      </IconButton>
      {open && (
        <Modal
          open
          onClose={close}
          title="What’s new"
          description={latestUpdate.title}
          icon={<SparklesIcon className="h-6 w-6" />}
          footerStart={
            <div className="space-y-1.5">
              <p>Step {index + 1} of {steps.length}</p>
              <div className="flex gap-1" aria-hidden="true">
                {steps.map((item, position) => (
                  <span key={item.id} className={`h-1 w-5 rounded-full ${position <= index ? "bg-primary" : "bg-border"}`} />
                ))}
              </div>
            </div>
          }
          footer={
            <>
              {previous && <Button variant="ghost" onClick={() => setStepId(previous.id)}>Back</Button>}
              {next
                ? <Button onClick={() => setStepId(next.id)}>Next</Button>
                : <Button onClick={close}>Done</Button>}
            </>
          }
        >
          <div className="space-y-4" aria-live="polite">
            <div>
              <h3 className="text-lg font-semibold">{step.title}</h3>
              <p className="mt-1 text-sm text-text-secondary">{step.description}</p>
            </div>
            <ol className="list-decimal space-y-2 pl-5 text-sm">
              {step.instructions.map((instruction) => <li key={instruction}>{instruction}</li>)}
            </ol>
            <Link to={step.path} onClick={close} className="inline-flex items-center gap-2 text-sm font-medium text-primary hover:underline">
              {step.link}
              <ArrowRightIcon className="h-4 w-4" aria-hidden="true" />
            </Link>
          </div>
        </Modal>
      )}
    </>
  );
}

export default function UpdateTutorial() {
  const userId = useAuthStore((state) => state.userId);
  return userId ? <Tutorial key={userId} userId={userId} /> : null;
}
