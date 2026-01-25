import { DocsThemeConfig } from 'nextra-theme-docs'

const basePath = process.env.NEXT_PUBLIC_BASE_PATH || ''

const config: DocsThemeConfig = {
  logo: (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
      <img src={`${basePath}/bunny-logo.svg`} alt="BunnyDB" width={28} height={28} />
      <span style={{ fontWeight: 700, fontSize: '1.1rem' }}>BunnyDB</span>
    </div>
  ),
  project: {
    link: 'https://github.com/Harshil-Jani/bunnyDB',
  },
  docsRepositoryBase: 'https://github.com/Harshil-Jani/bunnyDB/tree/main/docs',
  footer: {
    component: null,
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta name="description" content="BunnyDB — Fast, focused PostgreSQL-to-PostgreSQL CDC replication" />
      <meta name="og:title" content="BunnyDB Documentation" />
      <link rel="icon" href={`${basePath}/favicon.svg`} type="image/svg+xml" />
    </>
  ),
  color: {
    hue: 211,
    saturation: 62,
  },
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
  },
  toc: {
    float: true,
    backToTop: true,
  },
  navigation: {
    prev: true,
    next: true,
  },
  editLink: {
    content: 'Edit this page on GitHub →',
  },
  feedback: {
    content: 'Question? Give us feedback →',
    labels: 'docs-feedback',
  },
}

export default config
