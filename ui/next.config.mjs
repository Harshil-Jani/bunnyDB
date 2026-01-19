/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  env: {
    BUNNY_API_URL: process.env.BUNNY_API_URL || 'http://localhost:8112',
  },
};

export default nextConfig;
