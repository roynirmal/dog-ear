/** @type {import('next').NextConfig} */
const nextConfig = {
  // A static export that PocketBase serves from backend/pb_public (make up).
  output: 'export',
  images: {
    unoptimized: true,
  },
}

export default nextConfig
