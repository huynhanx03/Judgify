import { text } from "@/i18n/text";

export default function AboutPage() {
  return (
    <section className="relative isolate min-h-[80svh] w-full overflow-hidden rounded-2xl border border-border bg-surface shadow-sm motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200">
      <div
        aria-hidden="true"
        className="absolute inset-0 -z-20 bg-cover bg-center opacity-[0.08] grayscale dark:opacity-[0.12]"
        style={{ backgroundImage: "url('/images/about_bg.png')" }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 -z-10 bg-gradient-to-b from-background/95 via-background/75 to-background"
      />
      <div aria-hidden="true" className="surface-brand-radial pointer-events-none absolute inset-0 -z-10" />

      <div className="mx-auto flex w-full max-w-5xl flex-col items-center px-4 py-16 sm:px-8 sm:py-24 lg:py-28">
        <header className="max-w-3xl space-y-5 text-center motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200">
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-cultivation sm:text-sm">
            {text("ABOUT.MOTTO")}
          </p>
          <h1 className="font-heading text-4xl font-black tracking-tight text-foreground sm:text-6xl lg:text-7xl">
            {text("ABOUT.BRAND")}
          </h1>
          <div className="mx-auto h-px w-32 bg-gradient-to-r from-transparent via-primary to-transparent sm:w-48" />
        </header>

        <article className="mt-12 w-full space-y-8 rounded-2xl border border-border/80 bg-card/80 p-6 font-playfair text-base leading-8 text-card-foreground shadow-xl backdrop-blur-xl sm:mt-16 sm:p-10 sm:text-lg sm:leading-9 lg:p-14">
          <div className="space-y-6">
            <p className="first-letter:float-left first-letter:mr-3 first-letter:text-6xl first-letter:font-black first-letter:leading-none first-letter:text-primary">
              {text("ABOUT.GREETING")}
            </p>
            <p>{text("ABOUT.INTRO")}</p>
          </div>

          <blockquote className="rounded-r-xl border-l-4 border-primary/40 bg-primary/5 py-3 pl-5 pr-4 italic sm:pl-7">
            {text("ABOUT.WAY")}
          </blockquote>

          <p>
            {text("ABOUT.CHALLENGE_PREFIX")}{" "}
            <span className="font-bold text-status-success text-shadow-success">
              <q>{text("ABOUT.ACCEPTED")}</q>
            </span>{" "}
            {text("ABOUT.CHALLENGE_MIDDLE")}{" "}
            <span className="font-bold text-destructive underline decoration-destructive/50 underline-offset-4">
              <q>{text("ABOUT.WRONG_ANSWER")}</q>
            </span>{" "}
            {text("ABOUT.CHALLENGE_SUFFIX")}
          </p>

          <div className="space-y-6">
            <p>{text("ABOUT.PATHS")}</p>
            <p>{text("ABOUT.REMINDER")}</p>
          </div>

          <p>{text("ABOUT.JOURNEY")}</p>
        </article>

        <footer className="mt-12 max-w-3xl space-y-3 text-center sm:mt-16">
          <p className="font-playfair text-2xl font-black italic tracking-[0.08em] text-cultivation sm:text-3xl">
            {text("ABOUT.BLESSING_TITLE")}
          </p>
          <p className="text-sm uppercase tracking-[0.16em] text-muted-foreground sm:text-base">
            {text("ABOUT.BLESSING")}
          </p>
        </footer>
      </div>
    </section>
  );
}
