import type { MetadataRoute } from 'next'

export const dynamic = 'force-static'

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'Dog-ear',
    short_name: 'Dog-ear',
    description: 'Share what you’re reading with friends and keep track of books you lend and borrow.',
    start_url: '/',
    display: 'standalone',
    background_color: '#f8f4ec',
    theme_color: '#f8f4ec',
    icons: [
      { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml' },
      { src: '/apple-icon.png', sizes: '180x180', type: 'image/png' },
    ],
  }
}
