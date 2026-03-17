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
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]"></div>
      <div className="absolute left-0 right-0 top-0 -z-10 m-auto h-[310px] w-[310px] rounded-full bg-primary/20 opacity-20 blur-[100px]"></div>
      <div className="absolute bottom-0 left-1/4 -z-10 h-[250px] w-[250px] rounded-full bg-secondary/20 opacity-20 blur-[100px]"></div>

      <div className="z-10 w-full max-w-md px-4 animate-in slide-in-from-bottom-4 duration-700">
        {children}
      </div>
    </div>
  );
}
