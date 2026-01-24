import { Tour } from 'nextstepjs';

export function getTourSteps(role: 'admin' | 'readonly'): Tour[] {
  const isAdmin = role === 'admin';

  const steps: Tour['steps'] = [
    {
      icon: '🐰',
      title: 'Welcome to BunnyDB',
      content: 'Postgres-to-Postgres CDC replication. Quick tour — under 30 seconds.',
      side: 'bottom',
      showControls: true,
      showSkip: true,
    },
    {
      icon: '🔗',
      title: 'Peers',
      content: isAdmin
        ? 'Add and manage your source & destination database connections.'
        : 'View your database connections here.',
      selector: '#nav-peers',
      side: 'bottom',
      showControls: true,
      showSkip: true,
      pointerPadding: 4,
      pointerRadius: 8,
    },
    {
      icon: '🪞',
      title: 'Mirrors',
      content: isAdmin
        ? 'Create and control replication jobs between your peers.'
        : 'View active replication jobs and their status.',
      selector: '#nav-mirrors',
      side: 'bottom',
      showControls: true,
      showSkip: true,
      pointerPadding: 4,
      pointerRadius: 8,
    },
    {
      icon: '⚙️',
      title: 'Settings',
      content: isAdmin
        ? 'Manage users, roles, and change passwords.'
        : 'Change your password here.',
      selector: '#nav-settings',
      side: 'bottom',
      showControls: true,
      showSkip: true,
      pointerPadding: 4,
      pointerRadius: 8,
    },
  ];

  if (isAdmin) {
    steps.push(
      {
        icon: '1️⃣',
        title: 'Add a Peer',
        content: 'Register your source and destination Postgres DBs.',
        selector: '#add-peer-btn',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/peers',
        pointerPadding: 6,
        pointerRadius: 8,
      },
      {
        icon: '2️⃣',
        title: 'Create a Mirror',
        content: 'Pick source + destination, select tables, go.',
        selector: '#create-mirror-btn',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/mirrors',
        pointerPadding: 6,
        pointerRadius: 8,
      },
      {
        icon: '3️⃣',
        title: 'Monitor & Control',
        content: 'Pause, resume, resync tables, sync schema, view logs.',
        selector: '#mirrors-list',
        side: 'top',
        showControls: true,
        showSkip: true,
        pointerPadding: 8,
        pointerRadius: 12,
      },
    );
  } else {
    steps.push(
      {
        icon: '👁️',
        title: 'View Peers',
        content: 'See all registered database connections and their details.',
        selector: '#nav-peers',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/peers',
        pointerPadding: 4,
        pointerRadius: 8,
      },
      {
        icon: '📊',
        title: 'Monitor Mirrors',
        content: 'Track replication status, table progress, and logs.',
        selector: '#nav-mirrors',
        side: 'bottom',
        showControls: true,
        showSkip: true,
        nextRoute: '/mirrors',
        pointerPadding: 4,
        pointerRadius: 8,
      },
    );
  }

  steps.push(
    {
      icon: '👤',
      title: 'Your Account',
      content: isAdmin
        ? 'You have full access. Create, modify, and delete resources.'
        : 'You have read-only access. Ask an admin to make changes.',
      selector: '#user-menu',
      side: 'bottom',
      showControls: true,
      showSkip: true,
      pointerPadding: 6,
      pointerRadius: 8,
    },
    {
      icon: '🎉',
      title: 'All set!',
      content: isAdmin
        ? 'Add Peers → Create Mirror → Monitor. Restart with the ? icon.'
        : 'Explore the dashboard. Restart this tour with the ? icon.',
      side: 'bottom',
      showControls: true,
      showSkip: false,
    },
  );

  return [{ tour: 'onboarding', steps }];
}
