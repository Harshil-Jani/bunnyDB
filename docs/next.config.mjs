import nextra from 'nextra'

const isGHPages = process.env.DEPLOY_TARGET === 'ghpages'

const withNextra = nextra({
  theme: 'nextra-theme-docs',
  themeConfig: './theme.config.tsx',
  defaultShowCopyCode: true,
})

export default withNextra({
  output: isGHPages ? 'export' : 'standalone',
  basePath: isGHPages ? '/bunnyDB/docs' : '',
  images: {
    unoptimized: true,
  },
  trailingSlash: isGHPages ? true : undefined,
})
