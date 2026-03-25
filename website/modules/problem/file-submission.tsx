"use client";

/**
 * File-based submission component.
 * User selects a language, attaches a code file, then submits.
 */

import { useState, useRef } from "react";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import type { Language } from "@/types/submission";
import { Loader2, Send, Upload, FileCode2, X } from "lucide-react";

interface FileSubmissionProps {
  onSubmit: (code: string, language: Language) => void;
  isSubmitting: boolean;
}

const LANGUAGES: { value: Language; label: string; extensions: string }[] = [
  { value: "cpp", label: "C++", extensions: ".cpp,.cc,.cxx,.c,.h" },
  { value: "java", label: "Java", extensions: ".java" },
  { value: "python", label: "Python", extensions: ".py" },
  { value: "go", label: "Go", extensions: ".go" },
];

export function FileSubmission({ onSubmit, isSubmitting }: FileSubmissionProps) {
  const [language, setLanguage] = useState<Language>("cpp");
  const [file, setFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const currentLang = LANGUAGES.find((l) => l.value === language)!;

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0];
    if (selected) setFile(selected);
  }

  function clearFile() {
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function handleSubmit() {
    if (!file) return;
    const code = await file.text();
    onSubmit(code, language);
  }

  return (
    <div className="flex items-center gap-2">
      {/* Language selector — dropdown, wider */}
      <select
        value={language}
        onChange={(e) => { setLanguage(e.target.value as Language); clearFile(); }}
        className="h-9 w-36 rounded-md border border-border/40 bg-muted/30 px-3 text-sm font-medium text-foreground cursor-pointer outline-none focus:ring-1 focus:ring-primary"
      >
        {LANGUAGES.map((lang) => (
          <option key={lang.value} value={lang.value}>
            {lang.label}
          </option>
        ))}
      </select>

      {/* File picker — wider */}
      <input
        ref={fileInputRef}
        type="file"
        accept={currentLang.extensions}
        onChange={handleFileChange}
        className="hidden"
      />

      {file ? (
        <div className="flex items-center gap-2 bg-muted/30 rounded-lg px-4 py-2 text-sm min-w-[180px]">
          <FileCode2 className="h-4 w-4 text-primary shrink-0" />
          <span className="text-foreground font-medium truncate">{file.name}</span>
          <button onClick={clearFile} className="text-muted-foreground hover:text-foreground cursor-pointer shrink-0">
            <X className="h-4 w-4" />
          </button>
        </div>
      ) : (
        <Button
          variant="outline"
          onClick={() => fileInputRef.current?.click()}
          className="gap-2 text-sm cursor-pointer border-dashed h-9 min-w-[180px]"
        >
          <Upload className="h-4 w-4" />
          {TEXT.PROBLEM.SELECT_FILE}
        </Button>
      )}

      {/* Submit — right next to file picker */}
      <Button
        onClick={handleSubmit}
        disabled={isSubmitting || !file}
        className="bg-primary hover:bg-primary/90 text-primary-foreground cursor-pointer gap-2 h-9 px-5 text-sm"
      >
        {isSubmitting ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          <Send className="h-4 w-4" />
        )}
        {isSubmitting ? TEXT.PROBLEM.SUBMITTING : TEXT.PROBLEM.SUBMIT}
      </Button>
    </div>
  );
}
