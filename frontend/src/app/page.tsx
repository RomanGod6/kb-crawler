'use client';

import dynamic from 'next/dynamic'
import { Suspense } from 'react';
import CrawlerDashboard from '../components/ui/crawler-dashboard';

// Dynamically import the dashboard with no SSR
const O365Dashboard = dynamic(
  () => import('../components/ui/crawler-dashboard'),
  { ssr: false }
);

export default function Home() {
  return (
    <main className="min-h-screen">
      <Suspense fallback={<div>Loading...</div>}>
        <CrawlerDashboard />
      </Suspense>
    </main>
  );
}