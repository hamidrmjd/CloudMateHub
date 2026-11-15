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
  id: "websites-0.29.1",
  title: "Website access through the Hub",
  steps: [
    {
      id: "open-assigned-website",
      title: "Open assigned websites",
      description: "Read assigned public pages using the Hub’s internet connection.",
      instructions: [
        "Open Resources → Websites.",
        "Select Open through Hub to view the page in a new tab. File downloads are disabled.",
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
