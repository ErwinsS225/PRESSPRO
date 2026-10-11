export type ViewMode =
  | "dashboard"
  | "orders"
  | "customers"
  | "services"
  | "superadmin"
  | "billing";

export type UserRole =
  | "superadmin"
  | "owner"
  | "manager"
  | "cashier"
  | "operator";

export interface User {
  id: string;
  tenant_id?: string;
  email: string;
  full_name: string;
  phone?: string;
  role: UserRole;
  status: string;
}

export interface OpeningHours {
  day: string;
  open: string;
  close: string;
  isClosed: boolean;
}

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  phone: string;
  city: string;
  currency: string;
  status: "active" | "trial" | "suspended";
  ownerEmail?: string;
  monthlyPlan?: string;
  openingHours?: OpeningHours[]; // Horaires d'ouverture du pressing
}

export interface AuthSession {
  token: string;
  user: User;
  tenant?: Tenant;
}

export type OrderStatus =
  | "received"
  | "washing"
  | "ironing"
  | "ready"
  | "delivered";
export type PaymentStatus = "unpaid" | "partially_paid" | "paid";

export interface OrderItem {
  id: string;
  serviceName: string;
  quantity: number;
  unitPrice: number;
  subtotal: number;
}

export interface Order {
  id: string;
  orderNumber: string;
  customerName: string;
  customerPhone: string;
  items: OrderItem[];
  status: OrderStatus;
  paymentStatus: PaymentStatus;
  totalAmount: number;
  paidAmount: number;
  receivedAt: string;
  dueDate: string;
  notes?: string;
}

export interface Customer {
  id: string;
  fullName: string;
  phone: string;
  neighborhood: string;
  totalOrders: number;
  lastVisit: string;
  balanceDue: number;
}

export type PricingModel = "piece" | "kilogram" | "subscription" | "contract";

export interface ServiceItem {
  id: string;
  name: string;
  article?: string;
  treatment?: string;
  category: string;
  price: number;
  unit: string;
  pricingModel?: PricingModel;
  durationHours: number;
  isActive?: boolean;
  isExpressAvailable?: boolean;
  expressSurcharge?: number;
  isPickupAvailable?: boolean;
  isOutsourced?: boolean;
  requiresQuote?: boolean;
  description?: string;
  notes?: string;
}

export interface SubscriptionPlan {
  id: string;
  name: string;
  monthlyPrice: number; // en FCFA
  quotaPieces: number; // ex: 15 pièces incluses
  collectionFrequency: string; // ex: 2 fois / mois
  description: string;
  isPopular?: boolean;
  overagePricePerPiece: number; // Prix par pièce en dépassement
  target: "Individuel" | "Couple" | "Famille" | "Professionnel";
  isActive: boolean;
}

export interface B2BContract {
  id: string;
  clientName: string;
  businessType:
    | "Hôtel"
    | "Restaurant"
    | "Clinique"
    | "Salon & Spa"
    | "Entreprise";
  billingType: "piece" | "kilogram" | "lot" | "monthly_fixed";
  ratePerUnit: number; // FCFA
  billingCycle: "Fin de mois" | "Bi-mensuel" | "À la livraison";
  contactPerson: string;
  phone: string;
  status: "active" | "pending" | "suspended";
}

export interface HealthCheckResult {
  endpoint: string;
  status: "ok" | "ready" | "unavailable" | "error" | "testing";
  httpCode?: number;
  latencyMs?: number;
  data?: any;
  error?: string;
  lastChecked?: string;
}
