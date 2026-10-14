# Garage UI - Frontend

Modern React frontend for Garage S3-compatible object storage management.

## Features

- **Dashboard** - Overview of storage metrics and recent activity
- **Bucket Management** - Create, view, and manage S3 buckets
- **Object Browser** - Navigate folders, upload/download files with drag-and-drop
- **Access Control** - Manage API keys and permissions
- **Dark/Light Mode** - System-aware theme with manual toggle
- **Data-Dense UI** - Optimized for power users with comprehensive tables

## Tech Stack

- **Framework**: React 19 + TypeScript
- **Build Tool**: Vite 7
- **Routing**: React Router v6
- **State Management**: Zustand
- **UI Components**: shadcn/ui (built on Radix UI primitives)
- **Styling**: Tailwind CSS v4
- **Forms**: React Hook Form + Zod
- **Tables**: shadcn/ui table primitives with built-in column sorting
- **File Upload**: React Dropzone
- **Icons**: Lucide React

## Getting Started

### Prerequisites

- Node.js 18+ and npm

### Installation

1. Install dependencies:
```bash
npm install
```

2. Create environment file:
```bash
cp .env.example .env
```

3. Update `.env` with your backend API URL:
```
VITE_API_URL=http://localhost:8080/api
```

### Development

Start the development server:
```bash
npm run dev
```

The app will be available at http://localhost:3000

### Building for Production

Build the application:
```bash
npm run build
```

Preview the production build:
```bash
npm run preview
```

## Project Structure

```
src/
├── components/
│   ├── ui/              # shadcn/ui base components
│   ├── layout/          # Layout components (Sidebar, Header)
│   ├── buckets/         # Bucket-specific components
│   ├── objects/         # Object browser components
│   └── access/          # Access control components
├── pages/               # Page components
│   ├── Dashboard.tsx
│   ├── Buckets.tsx
│   ├── Objects.tsx
│   └── AccessControl.tsx
├── lib/
│   ├── api.ts          # API client with mock data
│   └── utils.ts        # Utility functions
├── hooks/              # Custom React hooks
├── types/              # TypeScript type definitions
└── stores/             # Zustand stores
```

## API Integration

The app is currently using mock data. To connect to your Go backend:

1. Ensure your backend is running on `http://localhost:8080`
2. In `src/lib/api.ts`, uncomment the real API calls
3. Comment out or remove the mock data returns

Example:
```typescript
// Before (mock data)
list: async (): Promise<Bucket[]> => {
  return mockBuckets;
},

// After (real API)
list: async (): Promise<Bucket[]> => {
  const response = await api.get('/buckets');
  return response.data;
},
```

## Features by Page

### Dashboard
- Storage metrics overview
- Bucket count and object statistics
- Storage distribution visualization
- Recent buckets list

### Buckets
- List all buckets with search
- Create new buckets
- View bucket details
- Delete buckets
- Region information

### Objects
- Browse objects and folders
- Breadcrumb navigation
- Drag-and-drop file upload
- Multiple file selection
- Download objects
- Delete objects
- Object metadata

### Access Control
- API key management
- Create keys with permissions
- Activate/deactivate keys
- Copy access key IDs
- Permission configuration

## Theme Customization

The app uses CSS variables for theming. Customize colors in `src/index.css`:

```css
:root {
  --primary: 240 5.9% 10%;
  --secondary: 240 4.8% 95.9%;
  /* ... more variables */
}
```

## Contributing

This is a custom frontend for Garage storage. Feel free to extend with additional features:
- Analytics dashboards
- Versioning management
- Lifecycle policies UI
- Replication configuration
- Metrics visualization

## License

MIT
