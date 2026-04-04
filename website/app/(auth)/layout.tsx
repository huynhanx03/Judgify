/**
 * Auth layout — centered layout without sidebar for login/register pages.
 * Features a premium animated gradient background.
 */

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background">
      {/* Premium Background Effects */}
      <div
        className="absolute inset-0 z-0 bg-cover bg-center opacity-80"
        style={{
          backgroundImage: "url('/images/auth_bg.png')",
          animation: "pan 60s linear infinite alternate"
        }}
      />
      <div className="absolute inset-0 bg-black/40 z-0"></div>

      {/* Magic Aura */}
      <div className="absolute left-0 right-0 top-0 z-0 m-auto h-[310px] w-[310px] rounded-full bg-cyan-600/30 opacity-40 blur-[120px]"></div>
      <div className="absolute bottom-0 left-1/4 z-0 h-[250px] w-[250px] rounded-full bg-purple-600/30 opacity-40 blur-[120px]"></div>

      <div className="z-10 w-full max-w-6xl px-4 py-8 animate-in fade-in zoom-in-95 duration-1000">
        {children}
      </div>

      <style dangerouslySetInnerHTML={{__html: `
        @keyframes pan {
          0% { background-position: 0% 50%; background-size: 110%; }
          100% { background-position: 100% 50%; background-size: 115%; }
        }
      `}} />
    </div>
  );
}
