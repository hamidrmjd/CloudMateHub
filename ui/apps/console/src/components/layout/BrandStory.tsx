import { CommandLineIcon, ServerStackIcon, ShieldCheckIcon } from "@heroicons/react/24/outline";

export default function BrandStory() {
  return <aside className="cmh-story">
    <div className="cmh-kicker"><span /> YOUR INFRASTRUCTURE. CONNECTED.</div>
    <h2>Every machine.<br /><em>One place.</em></h2>
    <p className="cmh-story-lead">A clearer view of your infrastructure. A simpler way to reach it.</p>
    <div className="cmh-network" aria-hidden="true">
      <div className="cmh-network-line" />
      <div className="cmh-network-node"><ServerStackIcon /><span>YOUR DEVICES</span></div>
      <div className="cmh-network-hub"><CommandLineIcon /><span>CloudMateHub</span></div>
      <div className="cmh-network-node"><ShieldCheckIcon /><span>YOUR TEAM</span></div>
    </div>
    <div className="cmh-story-features">
      <div><CommandLineIcon /><span><strong>Access from anywhere</strong><small>SSH and a terminal, right in your browser.</small></span></div>
      <div><ServerStackIcon /><span><strong>Bring your fleet together</strong><small>Devices, sessions, and access in one workspace.</small></span></div>
      <div><ShieldCheckIcon /><span><strong>Your infrastructure, your control</strong><small>Self-hosted. Built for your team.</small></span></div>
    </div>
    <span className="cmh-story-foot">LESS FRICTION. MORE CONNECTION.</span>
  </aside>;
}
