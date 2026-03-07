import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AcceptInviteModal from './AcceptInviteModal';
import * as householdApi from '../api/household';

vi.mock('../api/household');

const mockAcceptInvitation = vi.mocked(householdApi.acceptInvitation);

const INVITE_TOKEN = 'invite-tok';
const AUTH_TOKEN = 'auth-tok';

function renderModal(onClose = vi.fn()) {
  return render(
    <AcceptInviteModal
      inviteToken={INVITE_TOKEN}
      authToken={AUTH_TOKEN}
      onClose={onClose}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AcceptInviteModal', () => {
  it('renders invitation description and action buttons', () => {
    mockAcceptInvitation.mockResolvedValue(undefined);
    renderModal();
    expect(screen.getByRole('heading', { name: /unidad familiar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /aceptar invitación/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cancelar/i })).toBeInTheDocument();
  });

  it('calls onClose when Cancelar is clicked', async () => {
    mockAcceptInvitation.mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderModal(onClose);

    await userEvent.click(screen.getByRole('button', { name: /cancelar/i }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockAcceptInvitation).not.toHaveBeenCalled();
  });

  it('calls onClose when close icon is clicked', async () => {
    mockAcceptInvitation.mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderModal(onClose);

    await userEvent.click(screen.getByRole('button', { name: /cerrar/i }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('calls acceptInvitation with correct tokens on accept', async () => {
    mockAcceptInvitation.mockResolvedValue(undefined);
    // Prevent reload from throwing in jsdom
    Object.defineProperty(window, 'location', {
      value: { ...window.location, reload: vi.fn() },
      writable: true,
      configurable: true,
    });
    renderModal();

    await userEvent.click(screen.getByRole('button', { name: /aceptar invitación/i }));

    await waitFor(() => {
      expect(mockAcceptInvitation).toHaveBeenCalledWith(INVITE_TOKEN, AUTH_TOKEN);
    });
  });

  it('shows error message when acceptInvitation fails', async () => {
    mockAcceptInvitation.mockRejectedValue(new Error('La invitación no existe o ha expirado'));
    renderModal();

    await userEvent.click(screen.getByRole('button', { name: /aceptar invitación/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/la invitación no existe o ha expirado/i);
    });
    // buttons re-enabled after error
    expect(screen.getByRole('button', { name: /aceptar invitación/i })).not.toBeDisabled();
  });

  it('disables buttons while loading', async () => {
    let resolve!: () => void;
    mockAcceptInvitation.mockReturnValue(new Promise<void>((r) => { resolve = r; }));
    renderModal();

    await userEvent.click(screen.getByRole('button', { name: /aceptar invitación/i }));

    expect(screen.getByRole('button', { name: /uniéndose/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /cancelar/i })).toBeDisabled();

    resolve();
  });

  it('closes when clicking overlay background', async () => {
    mockAcceptInvitation.mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderModal(onClose);

    const overlay = document.querySelector('.modal') as HTMLElement;
    await userEvent.click(overlay);

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
