"use client";

/**
 * Problem table component for the Arena page.
 * Clean, minimalistic UI with columns for status, Title, Difficulty, Tags, Acpt Rate, and Solved Count.
 * Now receives sorting state from parent (ArenaClient).
 */

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { DifficultyBadge } from "./difficulty-badge";
import { TEXT } from "@/constants/text";
import type { Problem } from "@/types/problem";
import { CheckCircle2, Circle, ArrowUpDown, ArrowDown, ArrowUp } from "lucide-react";

export type SortField = 'acceptance' | 'solved' | 'none';
export type SortDirection = 'asc' | 'desc';

interface ProblemTableProps {
  problems: (Problem & { acceptance: number, solved: number, isSolved: boolean })[];
  sortField: SortField;
  sortDirection: SortDirection;
  onSort: (field: SortField) => void;
}

export function ProblemTable({ problems, sortField, sortDirection, onSort }: ProblemTableProps) {
  if (problems.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-muted-foreground border border-dashed border-border rounded-2xl bg-background/50 animate-in fade-in duration-300">
        <div className="h-12 w-12 rounded-full bg-muted flex items-center justify-center mb-4">
          <Circle className="h-6 w-6 text-muted-foreground/50" />
        </div>
        <p className="text-lg font-medium">{TEXT.ARENA.NO_PROBLEMS}</p>
        <p className="text-sm mt-1">{TEXT.ARENA.NO_PROBLEMS_DESC}</p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-border/40 bg-card/60 backdrop-blur-xl overflow-hidden shadow-sm animate-in fade-in duration-500">
      <Table className="table-fixed">
        <TableHeader className="bg-transparent hover:bg-transparent">
          <TableRow className="border-border/40 hover:bg-transparent">
            <TableHead className="w-16 text-center">
              <span className="sr-only">Trạng Thái</span>
            </TableHead>
            <TableHead className="font-semibold w-auto">
              {TEXT.ARENA.PROBLEM_TITLE}
            </TableHead>
            <TableHead className="w-32 font-semibold">
              {TEXT.ARENA.DIFFICULTY}
            </TableHead>
            <TableHead className="w-[200px] font-semibold md:w-[250px]">{TEXT.ARENA.TAGS_TITLE}</TableHead>
            <TableHead 
              className="w-28 text-center font-semibold cursor-pointer select-none group hidden md:table-cell hover:bg-muted/30 transition-colors" 
              onClick={() => onSort('acceptance')}
            >
              <div className="flex items-center justify-center gap-1.5 group-hover:text-foreground transition-colors">
                {TEXT.ARENA.SORT_ACCEPTANCE}
                {sortField === 'acceptance' ? (
                  sortDirection === 'desc' ? <ArrowDown className="h-3.5 w-3.5 text-primary" /> : <ArrowUp className="h-3.5 w-3.5 text-primary" />
                ) : (
                  <ArrowUpDown className="h-3.5 w-3.5 opacity-0 group-hover:opacity-50" />
                )}
              </div>
            </TableHead>
            <TableHead 
              className="w-32 text-center font-semibold cursor-pointer select-none group hidden lg:table-cell hover:bg-muted/30 transition-colors" 
              onClick={() => onSort('solved')}
            >
               <div className="flex items-center justify-center gap-1.5 group-hover:text-foreground transition-colors">
                {TEXT.ARENA.SOLVED}
                {sortField === 'solved' ? (
                  sortDirection === 'desc' ? <ArrowDown className="h-3.5 w-3.5 text-primary" /> : <ArrowUp className="h-3.5 w-3.5 text-primary" />
                ) : (
                  <ArrowUpDown className="h-3.5 w-3.5 opacity-0 group-hover:opacity-50" />
                )}
              </div>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {problems.map((problem) => (
            <TableRow
              key={problem.id}
              className="group cursor-pointer transition-colors duration-200 hover:bg-muted/30 border-border/20 last:border-0"
            >
              <TableCell className="text-center">
                {problem.isSolved ? (
                  <CheckCircle2 className="mx-auto h-[20px] w-[20px] text-emerald-500 drop-shadow-[0_0_2px_rgba(16,185,129,0.5)]" />
                ) : (
                  <Circle className="mx-auto h-[20px] w-[20px] text-muted-foreground/30 group-hover:text-muted-foreground/50 transition-colors" />
                )}
              </TableCell>
              
              <TableCell className="font-medium group-hover:text-primary transition-colors text-base truncate max-w-0" title={problem.title}>
                {problem.title}
              </TableCell>
              
              <TableCell>
                <DifficultyBadge difficulty={problem.difficulty} />
              </TableCell>
              
              <TableCell>
                <div className="flex flex-wrap gap-1.5">
                  {problem.tags?.slice(0, 3).map((tag) => (
                    <Badge 
                      key={tag.id} 
                      variant="secondary" 
                      className="text-[12px] px-2 py-0.5 h-auto font-medium bg-violet-500/10 text-violet-600 dark:text-violet-400 border-0 hover:bg-violet-500/20 transition-colors rounded-full"
                    >
                      {tag.name}
                    </Badge>
                  ))}
                  {problem.tags && problem.tags.length > 3 && (
                    <Badge variant="secondary" className="text-[12px] px-2 py-0.5 h-auto font-medium bg-muted text-muted-foreground border-0 rounded-full">
                      +{problem.tags.length - 3}
                    </Badge>
                  )}
                </div>
              </TableCell>

              <TableCell className="text-center text-muted-foreground text-base font-medium hidden md:table-cell group-hover:text-foreground transition-colors">
                {problem.acceptance.toFixed(1)}%
              </TableCell>

              <TableCell className="text-center text-muted-foreground text-base font-medium hidden lg:table-cell group-hover:text-foreground transition-colors">
                {new Intl.NumberFormat('en-US').format(problem.solved)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
