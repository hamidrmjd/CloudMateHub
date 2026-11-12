import type { Action } from "@/utils/permission";

type TutorialStep = {
  id: string;
  title: string;
  description: string;
  instructions: readonly string[];
  path: string;
  link: string;
  instanceAdmin?: boolean;
  permission?: Action;
  requiresAccessPolicies?: boolean;
};

export const latestUpdate: { id: string; title: string; steps: readonly TutorialStep[] } = {
  id: "workspace-access-update-1",
  title: "Workspaces and access",
  steps: [
    {
      id: "workspaces",
      title: "Switch between workspaces",
      description: "Each workspace has its own devices, sessions, and team roles.",
      instructions: [
        "Open Workspaces in the sidebar.",
        "Choose Open workspace for a workspace you belong to.",
      ],
      path: "/workspaces",
      link: "Go to Workspaces",
    },
    {
      id: "workspace-members",
      title: "Create a workspace and add members",
      description: "Instance admins can see who belongs to each workspace.",
      instructions: [
        "Open Workspaces and select Create workspace.",
        "In Team, select Add Member and choose the user’s role.",
        "Return to Workspaces and select Show members to review the roster.",
      ],
      path: "/workspaces",
      link: "Go to Workspaces",
      instanceAdmin: true,
    },
    {
      id: "user-management",
      title: "Add local users and groups",
      description: "Create accounts and workspace groups from User Management.",
      instructions: [
        "Choose Add user for a local account or Add group for a new workspace.",
        "Open the workspace’s Team page to give a user access.",
      ],
      path: "/users",
      link: "Go to User Management",
      instanceAdmin: true,
    },
    {
      id: "temporary-ssh",
      title: "Set an end time for SSH access",
      description: "After the deadline, the rule stops granting new SSH connections.",
      instructions: [
        "Open SSH → Access Policies in a workspace that uses SSH identities.",
        "Create or edit an Allow rule and set Expires at in your local time.",
      ],
      path: "/access-policies",
      link: "Go to Access Policies",
      permission: "accessPolicy:edit",
      requiresAccessPolicies: true,
    },
    {
      id: "agent-connection",
      title: "Reconnect a device or container agent",
      description: "Resetting the agent tunnel may interrupt active SSH connections.",
      instructions: [
        "Open a device or container’s detail page.",
        "Select Reconnect agent to reset its tunnel, or Disconnect agent to pause it until you reconnect.",
      ],
      path: "/devices",
      link: "Go to Devices",
      permission: "device:remove",
    },
    {
      id: "retention",
      title: "Set recording retention",
      description: "Cleanup removes completed recordings while preserving session history.",
      instructions: [
        "Open Settings → Storage & Recordings.",
        "Set Retention days and save. Enter 0 to disable automatic cleanup.",
      ],
      path: "/settings/storage",
      link: "Go to Storage & Recordings",
      instanceAdmin: true,
    },
  ],
};
