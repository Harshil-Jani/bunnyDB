import { Tour } from 'nextstepjs';

export const tourSteps: Tour[] = [
  {
    tour: 'onboarding',
    steps: [
      // Step 1: Welcome (no selector — centered overlay)
      {
        icon: '🐰',
        title: 'Welcome to BunnyDB',
        content: (
          <div className="space-y-2">
            <p>BunnyDB is a PostgreSQL-to-PostgreSQL replication tool powered by Change Data Capture (CDC).</p>
            <p className="text-gray-500 dark:text-gray-400 text-sm">Let us walk you through the key features in under a minute.</p>
          </div>
        ),
        side: 'bottom',
        showControls: true,
        showSkip: true,
      },
      // Step 2: Nav — Peers link
      {
        icon: '🔗',
        title: 'Peers',
        content: (
          <p>Peers are your PostgreSQL connections — both source and destination databases. Start here to register your database credentials.</p>
        ),
        selector: '#nav-peers',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        pointerPadding: 4,
        pointerRadius: 8,
      },
      // Step 3: Nav — Mirrors link
      {
        icon: '🪞',
        title: 'Mirrors',
        content: (
          <p>Mirrors are active replication jobs. Each mirror continuously streams changes from a source peer to a destination peer in real-time.</p>
        ),
        selector: '#nav-mirrors',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        pointerPadding: 4,
        pointerRadius: 8,
      },
      // Step 4: Nav — Settings link
      {
        icon: '⚙️',
        title: 'Settings',
        content: (
          <p>Manage users and change your password. Admins can create new users with either admin or read-only roles.</p>
        ),
        selector: '#nav-settings',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        pointerPadding: 4,
        pointerRadius: 8,
      },
      // Step 5: Navigate to Peers page
      {
        icon: '1️⃣',
        title: 'Step 1: Add a Peer',
        content: (
          <div className="space-y-2">
            <p>Click <strong>Add Peer</strong> to register your source and destination PostgreSQL databases.</p>
            <p className="text-gray-500 dark:text-gray-400 text-sm">You'll need: host, port, username, password, database name, and SSL mode.</p>
          </div>
        ),
        selector: '#add-peer-btn',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/peers',
        pointerPadding: 6,
        pointerRadius: 8,
      },
      // Step 6: Test connection button
      {
        icon: '🧪',
        title: 'Test Connections',
        content: (
          <p>After adding a peer, use the <strong>Test</strong> button to verify connectivity before creating mirrors.</p>
        ),
        selector: '#peers-list',
        side: 'top',
        showControls: true,
        showSkip: true,
        pointerPadding: 8,
        pointerRadius: 12,
      },
      // Step 7: Navigate to Mirrors — Create Mirror
      {
        icon: '2️⃣',
        title: 'Step 2: Create a Mirror',
        content: (
          <div className="space-y-2">
            <p>Once peers are set, navigate to <strong>Mirrors</strong> and click <strong>Create Mirror</strong>.</p>
            <p className="text-gray-500 dark:text-gray-400 text-sm">Select your source and destination peers, pick the tables to replicate, and BunnyDB handles the rest.</p>
          </div>
        ),
        selector: '#create-mirror-btn',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/mirrors',
        pointerPadding: 6,
        pointerRadius: 8,
      },
      // Step 8: Mirrors list area
      {
        icon: '3️⃣',
        title: 'Step 3: Monitor & Control',
        content: (
          <div className="space-y-2">
            <p>Active mirrors appear here with real-time status. Click any mirror for detailed controls:</p>
            <ul className="text-sm text-gray-600 dark:text-gray-300 list-disc pl-4 space-y-1">
              <li><strong>Pause/Resume</strong> — temporarily stop replication</li>
              <li><strong>Resync</strong> — re-copy tables with zero downtime</li>
              <li><strong>Sync Schema</strong> — propagate DDL changes</li>
              <li><strong>Logs</strong> — searchable event history</li>
            </ul>
          </div>
        ),
        selector: '#mirrors-list',
        side: 'top',
        showControls: true,
        showSkip: true,
        pointerPadding: 8,
        pointerRadius: 12,
      },
      // Step 9: User menu
      {
        icon: '👤',
        title: 'Your Account',
        content: (
          <div className="space-y-2">
            <p>You're logged in here. Click <strong>Log Out</strong> to end your session.</p>
            <p className="text-gray-500 dark:text-gray-400 text-sm">Role-based access: Admins can create/modify peers and mirrors. Read-only users can view everything but can't make changes.</p>
          </div>
        ),
        selector: '#user-menu',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        pointerPadding: 6,
        pointerRadius: 8,
      },
      // Step 10: Finish
      {
        icon: '🎉',
        title: 'You\'re all set!',
        content: (
          <div className="space-y-2">
            <p>That's the full workflow: <strong>Add Peers → Create Mirror → Monitor</strong>.</p>
            <p className="text-gray-500 dark:text-gray-400 text-sm">You can restart this tour anytime from Settings. Happy replicating!</p>
          </div>
        ),
        side: 'bottom',
        showControls: true,
        showSkip: false,
      },
    ],
  },
];
