"use client";

import { useState } from "react";
import {
  Folder,
  FolderOpen,
  File,
  ChevronRight,
  ChevronDown,
  Trash2,
} from "lucide-react";

export interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
}

interface FileTreeItemProps {
  node: FileNode;
  onFileSelect: (path: string) => void;
  selectedPath: string;
  onDelete: (path: string, e: React.MouseEvent) => void;
}

export function FileTreeItem({
  node,
  onFileSelect,
  selectedPath,
  onDelete,
}: FileTreeItemProps) {
  const [isOpen, setIsOpen] = useState(false);

  if (node.isDir) {
    return (
      <div className="pl-1">
        <div className="group flex items-center justify-between hover:bg-white/5 rounded px-2">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="flex items-center gap-1.5 py-1.5 text-white/70 hover:text-white text-sm flex-1 text-left min-w-0 transition-colors"
          >
            {isOpen ? (
              <ChevronDown className="w-3.5 h-3.5 text-white/40 shrink-0" />
            ) : (
              <ChevronRight className="w-3.5 h-3.5 text-white/40 shrink-0" />
            )}
            {isOpen ? (
              <FolderOpen className="w-4 h-4 text-sky-400 shrink-0" />
            ) : (
              <Folder className="w-4 h-4 text-sky-400 shrink-0" />
            )}
            <span className="truncate">{node.name}</span>
          </button>
          <button
            onClick={(e) => onDelete(node.path, e)}
            className="opacity-0 group-hover:opacity-100 text-white/40 hover:text-red-400 p-0.5 rounded transition-opacity shrink-0 ml-1"
            title="Delete Folder"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
        {isOpen && node.children && (
          <div className="border-l border-white/5 ml-3.5 pl-1.5">
            {node.children.map((child) => (
              <FileTreeItem
                key={child.path}
                node={child}
                onFileSelect={onFileSelect}
                selectedPath={selectedPath}
                onDelete={onDelete}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  const isSelected = selectedPath === node.path;
  return (
    <div className="group flex items-center justify-between hover:bg-white/5 rounded transition-all">
      <button
        onClick={() => onFileSelect(node.path)}
        className={`flex items-center gap-2 py-1.5 pl-6 flex-1 text-sm text-left min-w-0 transition-all ${
          isSelected
            ? "text-primary font-semibold border-l-2 border-primary"
            : "text-white/60 hover:text-white"
        }`}
      >
        <File
          className={`w-3.5 h-3.5 shrink-0 ${isSelected ? "text-primary" : "text-white/40"}`}
        />
        <span className="truncate">{node.name}</span>
      </button>
      <button
        onClick={(e) => onDelete(node.path, e)}
        className="opacity-0 group-hover:opacity-100 text-white/40 hover:text-red-400 p-0.5 rounded transition-opacity shrink-0 mr-2 ml-1"
        title="Delete File"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
    </div>
  );
}
