import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ChangePasswordModal from './ChangePasswordModal';

vi.mock('../api/auth', () => ({
  changePassword: vi.fn(),
}));

import { changePassword } from '../api/auth';
const mockChangePassword = vi.mocked(changePassword);

const mockOnClose = vi.fn();

function renderModal() {
  return render(<ChangePasswordModal token="test-token" onClose={mockOnClose} />);
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ChangePasswordModal', () => {
  it('renders the change password form', () => {
    renderModal();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Cambiar contraseña')).toBeInTheDocument();
    expect(screen.getByLabelText('Contraseña actual')).toBeInTheDocument();
    expect(screen.getByLabelText('Nueva contraseña')).toBeInTheDocument();
  });

  it('calls onClose when clicking the close button', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }));
    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when clicking the overlay', () => {
    renderModal();
    fireEvent.click(screen.getByRole('dialog'));
    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it('calls changePassword with the form values and token', async () => {
    mockChangePassword.mockResolvedValue(undefined);
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Contraseña actual'), { target: { value: 'oldpass1' } });
    fireEvent.change(screen.getByLabelText('Nueva contraseña'), { target: { value: 'newpass123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(mockChangePassword).toHaveBeenCalledWith('oldpass1', 'newpass123', 'test-token'),
    );
  });

  it('shows success message after a successful change', async () => {
    mockChangePassword.mockResolvedValue(undefined);
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Contraseña actual'), { target: { value: 'oldpass1' } });
    fireEvent.change(screen.getByLabelText('Nueva contraseña'), { target: { value: 'newpass123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('Contraseña actualizada correctamente'),
    );
  });

  it('shows error when the current password is incorrect', async () => {
    mockChangePassword.mockRejectedValue(new Error('La contraseña actual es incorrecta'));
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Contraseña actual'), { target: { value: 'wrong' } });
    fireEvent.change(screen.getByLabelText('Nueva contraseña'), { target: { value: 'newpass123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('La contraseña actual es incorrecta'),
    );
  });

  it('disables the submit button while loading', async () => {
    mockChangePassword.mockImplementation(() => new Promise(() => {})); // never resolves
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Contraseña actual'), { target: { value: 'oldpass1' } });
    fireEvent.change(screen.getByLabelText('Nueva contraseña'), { target: { value: 'newpass123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(container.querySelector('button[type="submit"]')).toBeDisabled(),
    );
  });
});
