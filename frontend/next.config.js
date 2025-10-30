/** @type {import('next').NextConfig} */

// Get the backend URL from env or fallback to localhost:4000
const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:4000';
const apiUrlObj = new URL(apiUrl);

const nextConfig = {
  images: {
    remotePatterns: [
      {
        protocol: apiUrlObj.protocol.replace(':', ''),
        hostname: apiUrlObj.hostname,
        port: apiUrlObj.port,
        pathname: '/uploads/**',
      },
      // Add localhost for local development
      {
        protocol: 'http',
        hostname: 'localhost',
        port: '4000',
        pathname: '/uploads/**',
      },
      {
        protocol: 'https',
        hostname: 'images.unsplash.com',
        port: '',
        pathname: '/**',
      },
    ],
  },
  async rewrites() {
    return [
      {
        source: '/uploads/:path*',
        destination: `${apiUrl}/uploads/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
