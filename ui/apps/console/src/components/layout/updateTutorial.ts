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

/** One-time walkthrough for the current user-facing feature release. */
export const latestUpdate: { id: string; title: string; steps: readonly TutorialStep[] } = {
  id: "websites-0.30.0",
  title: "Interactive websites through the Hub",
  steps: [
    {
      id: "open-assigned-website",
      title: "Open assigned websites",
      description: "Browse assigned public websites using the Hub’s internet connection.",
      instructions: [
        "Open Resources → Websites.",
        "Select Open through Hub to start your private browser in a new tab.",
        "Click links, type in search fields, and scroll. Use Back, Forward, and Reload above the website.",
        "Downloads and uploads are disabled. Close the tab when finished; inactive sessions expire after five minutes.",
      ],
      path: "/websites",
      link: "Go to Websites",
    },
    {
      id: "configure-website-access",
      title: "Set who can open a website",
      description: "Administrators can share a website with a workspace or selected users and groups.",
      instructions: [
        "Select Add website, or use Add NVIDIA example to try the form.",
        "Turn on Proxy traffic through the Hub and choose who can see the website.",
        "Use the username search for individual assignments, then save.",
      ],
      path: "/websites",
      link: "Go to Websites",
      instanceAdmin: true,
    },
  ],
};
