import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Analytics from './Analytics';
import type { AnalyticsResult } from '../types';

vi.mock('../api/products', () => ({
  getAnalytics: vi.fn(),
}));

vi.mock('./ProductImage', () => ({
  default: ({ name }: { name: string }) => <div data-testid="product-image">{name}</div>,
}));

vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: ({ children, onClick }: { children: React.ReactNode; onClick?: (data: unknown) => void }) => (
    <div
      data-testid="bar"
      onClick={() => onClick?.({ key: '2024-01', label: 'ene 24', value: 0.0, ticketCount: 1, tickets: [] })}
    >
      {children}
    </div>
  ),
  Cell: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ReferenceLine: () => null,
}));

import { getAnalytics } from '../api/products';

const mockAnalytics: AnalyticsResult = {
  mostPurchased: [
    { id: 'leche', name: 'LECHE ENTERA', purchaseCount: 5, currentPrice: 0.89 },
    { id: 'pan', name: 'PAN INTEGRAL', purchaseCount: 3, currentPrice: 1.25 },
  ],
  biggestIncreases: [
    { id: 'aceite', name: 'ACEITE OLIVA', firstPrice: 3.00, currentPrice: 6.00, increasePercent: 100.0 },
    { id: 'yogur', name: 'YOGUR NATURAL', firstPrice: 0.40, currentPrice: 0.60, increasePercent: 50.0 },
  ],
  basketInflation: [
    {
      date: '2024-01-15', inflationPercent: 0.0, productsCount: 1,
      products: [{ productId: 'leche', productName: 'LECHE ENTERA', firstPrice: 0.89, currentPrice: 0.89, inflationPercent: 0.0 }],
    },
    {
      date: '2024-06-20', inflationPercent: 5.3, productsCount: 2,
      products: [
        { productId: 'aceite', productName: 'ACEITE OLIVA', firstPrice: 3.00, currentPrice: 6.00, inflationPercent: 100.0 },
        { productId: 'leche', productName: 'LECHE ENTERA', firstPrice: 0.89, currentPrice: 0.89, inflationPercent: 0.0 },
      ],
    },
  ],
};

const emptyAnalytics: AnalyticsResult = {
  mostPurchased: [],
  biggestIncreases: [],
  basketInflation: [],
};

beforeEach(() => {
  vi.resetAllMocks();
});

describe('Analytics', () => {
  it('shows a loading indicator while data is being fetched', () => {
    vi.mocked(getAnalytics).mockReturnValue(new Promise(() => {}));
    render(<Analytics onSelectProduct={vi.fn()} />);
    expect(screen.getByRole('status', { hidden: true }) ?? document.querySelector('[aria-busy="true"]')).toBeTruthy();
  });

  it('renders both analytics sections with data', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Productos más comprados')).toBeInTheDocument();
      expect(screen.getByText('Mayor subida de precio')).toBeInTheDocument();
    });
  });

  it('shows the most purchased products with name and count', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('LECHE ENTERA')).toBeInTheDocument();
      expect(screen.getByText('PAN INTEGRAL')).toBeInTheDocument();
    });

    expect(screen.getByText('5 veces')).toBeInTheDocument();
    expect(screen.getByText('3 veces')).toBeInTheDocument();
  });

  it('shows the products with the biggest price increase', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('ACEITE OLIVA')).toBeInTheDocument();
      expect(screen.getByText('YOGUR NATURAL')).toBeInTheDocument();
    });

    expect(screen.getByText('+100,0%')).toBeInTheDocument();
    expect(screen.getByText('+50,0%')).toBeInTheDocument();
  });

  it('shows empty message when there is no most-purchased data', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(emptyAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      const emptyMessages = screen.getAllByText('Aún no hay datos suficientes.');
      expect(emptyMessages).toHaveLength(3);
    });
  });

  it('shows an error message when the API fails', async () => {
    vi.mocked(getAnalytics).mockRejectedValue(new Error('Network error'));
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByRole('alert')).toHaveTextContent('Network error');
    });
  });

  it('calls onSelectProduct when clicking a most-purchased product', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    const onSelectProduct = vi.fn();
    render(<Analytics onSelectProduct={onSelectProduct} />);

    await waitFor(() => screen.getByText('LECHE ENTERA'));

    await userEvent.click(screen.getByText('LECHE ENTERA'));
    expect(onSelectProduct).toHaveBeenCalledWith('leche');
  });

  it('calls onSelectProduct when clicking a product with the biggest price increase', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    const onSelectProduct = vi.fn();
    render(<Analytics onSelectProduct={onSelectProduct} />);

    await waitFor(() => screen.getByText('ACEITE OLIVA'));

    await userEvent.click(screen.getByText('ACEITE OLIVA'));
    expect(onSelectProduct).toHaveBeenCalledWith('aceite');
  });

  it('shows the price range (first price → current price) for increases', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('ACEITE OLIVA'));

    // Prices must appear formatted in euros
    const priceTexts = screen.getAllByText(/3,00\s*€|6,00\s*€/);
    expect(priceTexts.length).toBeGreaterThanOrEqual(2);
  });

  it('shows the correct price range for yogurt', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('YOGUR NATURAL'));

    const priceTexts = screen.getAllByText(/0,40\s*€|0,60\s*€/);
    expect(priceTexts.length).toBeGreaterThanOrEqual(2);
  });

  it('shows "1 vez" in singular when purchaseCount is 1', async () => {
    vi.mocked(getAnalytics).mockResolvedValue({
      mostPurchased: [
        { id: 'sal', name: 'SAL FINA', purchaseCount: 1, currentPrice: 0.45 },
      ],
      biggestIncreases: [],
      basketInflation: [],
    });
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('SAL FINA'));
    expect(screen.getByText('1 vez')).toBeInTheDocument();
  });

  it('renders all three section headers', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Productos más comprados')).toBeInTheDocument();
      expect(screen.getByText('Mayor subida de precio')).toBeInTheDocument();
      expect(screen.getByText('Inflación de tu cesta')).toBeInTheDocument();
    });
  });

  it('opens a section when its header is clicked', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('Mayor subida de precio'));
    const header = screen.getByText('Mayor subida de precio').closest('button')!;
    expect(header).toHaveAttribute('aria-expanded', 'false');
    await userEvent.click(header);
    expect(header).toHaveAttribute('aria-expanded', 'true');
  });

  it('shows empty message when basket inflation has fewer than 2 points', async () => {
    vi.mocked(getAnalytics).mockResolvedValue({
      ...mockAnalytics,
      basketInflation: [{ date: '2024-01-01', inflationPercent: 0, productsCount: 5, products: [] }],
    });
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('Inflación de tu cesta'));
    const emptyMessages = screen.getAllByText('Aún no hay datos suficientes.');
    expect(emptyMessages.length).toBeGreaterThanOrEqual(1);
  });

  it('shows the latest inflation badge when data has 2+ points', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('Media periodo más reciente'));
    expect(screen.getByText('+5,3%')).toBeInTheDocument();
  });

  it('shows negative inflation with correct sign', async () => {
    vi.mocked(getAnalytics).mockResolvedValue({
      ...mockAnalytics,
      basketInflation: [
        { date: '2024-01-01', inflationPercent: 0, productsCount: 1, products: [{ productId: 'p1', productName: 'P1', firstPrice: 1, currentPrice: 1, inflationPercent: 0 }] },
        { date: '2024-06-01', inflationPercent: -2.5, productsCount: 1, products: [{ productId: 'p1', productName: 'P1', firstPrice: 1, currentPrice: 0.975, inflationPercent: -2.5 }] },
      ],
    });
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('Media periodo más reciente'));
    expect(screen.getByText('-2,5%')).toBeInTheDocument();
  });

  it('shows ticket list after clicking a bar', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    // Open basketInflation section
    await waitFor(() => screen.getByText('Inflación de tu cesta'));
    await userEvent.click(screen.getByText('Inflación de tu cesta').closest('button')!);

    await waitFor(() => screen.getByTestId('bar'));
    await userEvent.click(screen.getByTestId('bar'));

    await waitFor(() => {
      expect(screen.getByText(/Tiquets de/)).toBeInTheDocument();
    });
  });

  it('shows period selector with three buttons', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mensual' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Trimestral' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Anual' })).toBeInTheDocument();
    });
  });

  it('Mensual button is pressed by default', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mensual' })).toHaveAttribute('aria-pressed', 'true');
    });
  });

  it('shows per-ticket rows after clicking a bar, then product breakdown on ticket click', async () => {
    const analyticsWithProducts: typeof mockAnalytics = {
      ...mockAnalytics,
      basketInflation: [
        {
          date: '2024-01-15', inflationPercent: 0.0, productsCount: 1,
          products: [{ productId: 'leche', productName: 'LECHE ENTERA', firstPrice: 0.89, currentPrice: 0.89, inflationPercent: 0.0 }],
        },
        {
          date: '2024-01-20', inflationPercent: 5.3, productsCount: 1,
          products: [{ productId: 'aceite', productName: 'ACEITE OLIVA', firstPrice: 3.00, currentPrice: 6.00, inflationPercent: 100.0 }],
        },
      ],
    };
    vi.mocked(getAnalytics).mockResolvedValue(analyticsWithProducts);
    render(<Analytics onSelectProduct={vi.fn()} />);

    // Open basketInflation section
    await waitFor(() => screen.getByText('Inflación de tu cesta'));
    await userEvent.click(screen.getByText('Inflación de tu cesta').closest('button')!);

    await waitFor(() => screen.getByTestId('bar'));
    await userEvent.click(screen.getByTestId('bar'));

    await waitFor(() => screen.getByText(/Tiquets de/));

    // Both tickets are in the same month — both appear as expandable ticket rows
    const ticketRowButtons = Array.from(document.querySelectorAll<HTMLElement>('.analytics__ticket-row'));
    expect(ticketRowButtons.length).toBeGreaterThanOrEqual(1);

    // Click first ticket row to expand products
    await userEvent.click(ticketRowButtons[0]);
    await waitFor(() => {
      const rows = document.querySelectorAll('.analytics__ticket-products .analytics-row');
      expect(rows.length).toBeGreaterThanOrEqual(1);
    });
  });
});
