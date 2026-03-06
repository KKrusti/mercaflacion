import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProductDetail from './ProductDetail';
import * as productsApi from '../api/products';
import type { Product } from '../types';

vi.mock('../api/products');
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  LineChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const mockProduct: Product = {
  id: '1',
  name: 'LECHE ENTERA HACENDADO 1L',
  category: 'Lácteos',
  currentPrice: 0.89,
  priceHistory: [
    { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona' },
    { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ProductDetail', () => {
  it('shows "Cargando producto..." while loading', () => {
    vi.mocked(productsApi.getProduct).mockReturnValue(new Promise(() => {}));
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    expect(screen.getByText('Cargando producto...')).toBeInTheDocument();
  });

  it('shows the product name after loading', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText('LECHE ENTERA HACENDADO 1L')).toBeInTheDocument()
    );
  });

  it('shows the product category', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Lácteos')).toBeInTheDocument());
  });

  it('shows the current price formatted in the header', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => {
      const priceEl = document.querySelector('.detail-header .price');
      expect(priceEl).toHaveTextContent('0,89 €');
    });
  });

  it('shows the price history table', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('Evolución del precio'));
    expect(screen.getByText('Fecha')).toBeInTheDocument();
    expect(screen.getByText('Precio')).toBeInTheDocument();
    expect(screen.getByText('Tienda')).toBeInTheDocument();
  });

  it('shows the stores in the table', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('Evolución del precio'));
    expect(screen.getAllByText('Mercadona')).toHaveLength(2);
  });

  it('shows a not found message when getProduct fails', async () => {
    vi.mocked(productsApi.getProduct).mockRejectedValue(new Error('Not found'));
    render(<ProductDetail productId="9999" onBack={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText('Producto no encontrado')).toBeInTheDocument()
    );
  });

  it('calls onBack when the back button is pressed', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    const onBack = vi.fn();
    render(<ProductDetail productId="1" onBack={onBack} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByRole('button', { name: /volver/i }));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it('calls getProduct with the correct ID', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    expect(productsApi.getProduct).toHaveBeenCalledWith('1');
  });

  it('shows a price-change badge when there are at least 2 price records', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    // 0.79 → 0.89 = +12.7%
    expect(screen.getByText('+12,7%')).toBeInTheDocument();
  });

  it('shows the badge in red (--up modifier) when price increased', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('+12,7%'));
    const badge = screen.getByText('+12,7%').closest('.price-change-badge');
    expect(badge).toHaveClass('price-change-badge--up');
  });

  it('shows the badge in green (--down modifier) when price decreased', async () => {
    const cheaperProduct: Product = {
      ...mockProduct,
      currentPrice: 0.69,
      priceHistory: [
        { date: '2025-01-15T00:00:00Z', price: 0.89, store: 'Mercadona' },
        { date: '2025-09-22T00:00:00Z', price: 0.69, store: 'Mercadona' },
      ],
    };
    vi.mocked(productsApi.getProduct).mockResolvedValue(cheaperProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    // 0.89 → 0.69 = -22.5%
    const badge = screen.getByText('-22,5%').closest('.price-change-badge');
    expect(badge).toHaveClass('price-change-badge--down');
  });

  it('does not show the badge when there is only one price record', async () => {
    const singleRecordProduct: Product = {
      ...mockProduct,
      priceHistory: [{ date: '2025-01-15T00:00:00Z', price: 0.89, store: 'Mercadona' }],
    };
    vi.mocked(productsApi.getProduct).mockResolvedValue(singleRecordProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    expect(document.querySelector('.price-change-badge')).not.toBeInTheDocument();
  });

  describe('ImageEditor', () => {
    it('shows the edit image button after loading', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      expect(screen.getByRole('button', { name: 'Cambiar imagen del producto' })).toBeInTheDocument();
    });

    it('shows URL input when the edit button is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      expect(screen.getByLabelText('URL de imagen del producto')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Guardar imagen' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument();
    });

    it('hides the input when cancel is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cambiar imagen del producto' })).toBeInTheDocument();
    });

    it('calls updateProductImage and closes input on success', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      vi.mocked(productsApi.updateProductImage).mockResolvedValue('https://prod.mercadona.com/img/leche.jpg');
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      fireEvent.change(screen.getByLabelText('URL de imagen del producto'), {
        target: { value: 'https://prod.mercadona.com/img/leche.jpg' },
      });
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      await waitFor(() =>
        expect(productsApi.updateProductImage).toHaveBeenCalledWith(
          '1',
          'https://prod.mercadona.com/img/leche.jpg',
        ),
      );
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
    });

    it('shows an error alert if saving fails', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      vi.mocked(productsApi.updateProductImage).mockRejectedValue(new Error('Network error'));
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      fireEvent.change(screen.getByLabelText('URL de imagen del producto'), {
        target: { value: 'https://example.com/img.jpg' },
      });
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      await waitFor(() =>
        expect(screen.getByRole('alert')).toBeInTheDocument(),
      );
    });

    it('shows validation error when saving with empty URL', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      expect(screen.getByRole('alert')).toHaveTextContent('Introduce una URL');
      expect(productsApi.updateProductImage).not.toHaveBeenCalled();
    });

    it('closes input on Escape key', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.keyboard('{Escape}');
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
    });
  });
});
