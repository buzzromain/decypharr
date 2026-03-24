import { createBrowserRouter } from 'react-router-dom'
import AppLayout from './layouts/AppLayout'
import AuthLayout from './layouts/AuthLayout'
import QueuePage from './pages/Queue'
import BrowsePage from './pages/Browse'
import RepairPage from './pages/Repair'
import SettingsPage from './pages/Settings'
import StatsPage from './pages/Stats'
import LogsPage from './pages/Logs'
import LibraryPage from './pages/Library'
import MediaDetailPage from './pages/MediaDetail'
import LoginPage from './pages/Login'
import RegisterPage from './pages/Register'
import SetupPage from './pages/Setup'

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { path: '/',         element: <QueuePage /> },
      { path: '/repair',   element: <RepairPage /> },
      { path: '/stats',    element: <StatsPage /> },
      { path: '/settings', element: <SettingsPage /> },
      { path: '/browse',   element: <BrowsePage /> },
      { path: '/library',  element: <LibraryPage /> },
      { path: '/library/:hash', element: <MediaDetailPage /> },
      { path: '/logs',     element: <LogsPage /> },
    ],
  },
  {
    element: <AuthLayout />,
    children: [
      { path: '/login',    element: <LoginPage /> },
      { path: '/register', element: <RegisterPage /> },
    ],
  },
  {
    path: '/setup',
    element: <SetupPage />,
  },
])
