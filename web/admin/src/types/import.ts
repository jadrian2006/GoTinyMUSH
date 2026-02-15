export type FileRole = 'flatfile' | 'comsys' | 'main_config' | 'alias_config' | 'text' | 'dict' | 'unknown'

export interface DiscoveredFile {
  path: string
  role: FileRole
  confidence: 'high' | 'medium' | 'low' | 'manual'
  size: number
  reason: string
}

export interface ImportSessionState {
  active: boolean
  source?: 'flatfile' | 'gotinymush_archive' | 'foreign_archive'
  staging_dir?: string
  files?: DiscoveredFile[]
  config_ready?: boolean
  comsys_parsed?: boolean
  validation_done?: boolean
  ready_to_commit?: boolean
  upload_file?: string
  object_count?: number
  attr_defs?: number
  total_attrs?: number
  type_counts?: Record<string, number>
  manifest?: any
  channel_count?: number
  alias_count?: number
  text_files?: string[]
  dict_files?: string[]
  alias_files?: string[]
  has_config?: boolean
}

export interface ChannelSummary {
  name: string
  owner: number
  description: string
  num_sent: number
}
