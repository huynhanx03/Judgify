import { text } from "@/i18n/text";
import { PublicPageLead } from "@/components/public-page-lead";

export default function AboutPage() {
  return (
    <div className="space-y-10 pb-16 sm:space-y-12 sm:pb-20">
      <PublicPageLead
        art="about"
        eyebrow={text("ABOUT.MOTTO")}
        title={text("ABOUT.BRAND")}
        description={text("ABOUT.DESCRIPTION")}
        className="motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200"
      />

      <article className="max-w-5xl space-y-8 font-playfair text-base leading-8 text-foreground sm:text-lg sm:leading-9">
        <div className="space-y-6">
          <p className="first-letter:float-left first-letter:mr-3 first-letter:text-6xl first-letter:font-black first-letter:leading-none first-letter:text-primary">
            {text("ABOUT.GREETING")}
          </p>
          <p>{text("ABOUT.INTRO")}</p>
        </div>

        <blockquote className="border-l-2 border-primary/60 py-1 pl-5 italic text-muted-foreground sm:pl-6">
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

      <footer className="max-w-5xl border-t border-border/60 pt-8">
        <p className="font-playfair text-2xl font-bold italic tracking-[0.04em] text-cultivation sm:text-3xl">
          {text("ABOUT.BLESSING_TITLE")}
        </p>
        <p className="mt-2 text-sm tracking-wide text-muted-foreground sm:text-base">
          {text("ABOUT.BLESSING")}
        </p>
      </footer>
    </div>
  );
}
