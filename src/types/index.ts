export type ViewMode =
  | 'dashboard'
  | 'orders'
  | 'customers'
  | 'services'
  | 'superadmin'
  | 'system-health'
  | 'docs';

export type UserRole = 'superadmin' | 'owner' | 'manager' | 'cashier' | 'operator';

export interface User {
  id: string;
  tenant_id?: string;
  email: string;
  full_name: string;
  phone?: string;
  role: UserRole;
  status: string;
}

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  phone: string;
  city: string;
  currency: string;
  status: 'active' | 'trial' | 'suspended';
  ownerEmail?: string;
  monthlyPlan?: string;
}

export interface AuthSession {
  token: string;
  user: User;
  tenant?: Tenant;
}

export type OrderStatus = 'received' | 'washing' | 'ironing' | 'ready' | 'delivered';
export type PaymentStatus = 'unpaid' | 'partially_paid' | 'paid';

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

export interface ServiceItem {
  id: string;
  name: string;
  category: string;
  price: number;
  unit: string;
  durationHours: number;
}

export interface HealthCheckResult {
  endpoint: string;
  status: 'ok' | 'ready' | 'unavailable' | 'error' | 'testing';
  httpCode?: number;
  latencyMs?: number;
  data?: any;
  error?: string;
  lastChecked?: string;
}
