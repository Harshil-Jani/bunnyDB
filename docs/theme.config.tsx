import { DocsThemeConfig } from 'nextra-theme-docs'

const basePath = process.env.NEXT_PUBLIC_BASE_PATH || ''
const homeUrl = basePath ? '/bunnyDB' : '/'

const config: DocsThemeConfig = {
  logo: (
    <a href={homeUrl} style={{ display: 'flex', alignItems: 'center', gap: '8px', textDecoration: 'none', color: 'inherit' }}>
      <img src={`${basePath}/bunny-logo.svg`} alt="BunnyDB" width={28} height={28} />
      <span style={{ fontWeight: 700, fontSize: '1.1rem' }}>BunnyDB</span>
      <span style={{
        fontSize: '0.65rem',
        fontWeight: 500,
        padding: '2px 6px',
        borderRadius: '9999px',
        backgroundColor: 'rgba(34, 197, 94, 0.15)',
        color: 'rgb(22, 163, 74)'
      }}>v1.0.0</span>
    </a>
  ),
  logoLink: false,
  project: {
    link: 'https://github.com/Harshil-Jani/bunnyDB',
  },
  navbar: {
    extraContent: (
      <a
        href={homeUrl}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          padding: '4px 8px',
          fontSize: '14px',
          color: 'inherit',
          textDecoration: 'none',
          opacity: 0.7,
        }}
      >
        Home
      </a>
    ),
  },
  docsRepositoryBase: 'https://github.com/Harshil-Jani/bunnyDB/tree/main/docs',
  footer: {
    component: null,
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta name="description" content="BunnyDB - Fast, focused PostgreSQL-to-PostgreSQL CDC replication" />
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
