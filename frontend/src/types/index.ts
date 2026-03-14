export interface PriceRecord {
  recordId?: number; // DB primary key; present for records belonging to the authenticated user
  date: string;
  price: number;
  store?: string;
}

export interface Product {
  id: string;
  name: string;
  category?: string;
  imageUrl?: string;
  currentPrice: number;
  priceHistory: PriceRecord[];
}

export interface SearchResult {
  id: string;
  name: string;
  category?: string;
  imageUrl?: string;
  currentPrice: number;
  minPrice: number;
  maxPrice: number;
  lastPurchaseDate?: string;
}

export interface TicketUploadResult {
  invoiceNumber: string;
  linesImported: number;
}

export type TicketUploadItem =
  | { file: string; ok: true; result: TicketUploadResult }
  | { file: string; ok: false; error: string };

export interface TicketUploadSummary {
  total: number;
  succeeded: number;
  failed: number;
  items: TicketUploadItem[];
}

export interface MostPurchasedProduct {
  id: string;
  name: string;
  imageUrl?: string;
  purchaseCount: number;
  currentPrice: number;
}

export interface PriceIncreaseProduct {
  id: string;
  name: string;
  imageUrl?: string;
  firstPrice: number;
  currentPrice: number;
  increasePercent: number;
}

export interface AnalyticsResult {
  mostPurchased: MostPurchasedProduct[];
  biggestIncreases: PriceIncreaseProduct[];
}

export interface User {
  userId: number;
  username: string;
  email?: string;
  isAdmin: boolean;
}

export interface AuthState {
  user: User | null;
  token: string | null;
}
