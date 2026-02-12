export interface UserPublic {
  id: string;
  username: string;
  email: string;
  phone: string;
  company: string;
  position: string;
  avatar_url: string;
  is_online: boolean;
  last_seen_at: string;
  disabled_at?: string | null;
}

export interface Chat {
  id: string;
  chat_type: 'personal' | 'group' | 'notes';
  system_key?: string;
  name: string;
  description: string;
  avatar_url: string;
  created_by: string;
  created_at: string;
}

export interface Reaction {
  message_id: string;
  user_id: string;
  emoji: string;
  username?: string;
  created_at: string;
}

export interface Message {
  id: string;
  chat_id: string;
  sender_id: string;
  content: string;
  content_type: 'text' | 'image' | 'file' | 'voice' | 'system';
  file_url?: string;
  file_name?: string;
  file_size?: number;
  status: 'sent' | 'delivered' | 'read';
  reply_to_id?: string;
  edited_at?: string;
  is_deleted: boolean;
  created_at: string;
  sender?: UserPublic;
  reply_to?: Message;
  reactions?: Reaction[];
}

export interface ChatWithLastMessage {
  chat: Chat;
  last_message?: Message;
  members: UserPublic[];
  unread_count: number;
  muted?: boolean;
}

export interface PinnedMessage {
  chat_id: string;
  message_id: string;
  pinned_by: string;
  pinned_at: string;
  message?: Message;
}

export interface UserStats {
  user: UserPublic;
  avg_response_sec: number;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: UserPublic;
}

export interface FileUploadResponse {
  url: string;
  file_name: string;
  file_size: number;
  content_type: string;
}
