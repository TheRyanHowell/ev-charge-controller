export default function DashboardLoading() {
  return (
    <main className="min-h-screen bg-page-bg text-white flex items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4" />
        <p className="text-gray-400">Loading...</p>
      </div>
    </main>
  );
}
