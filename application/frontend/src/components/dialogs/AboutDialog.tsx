import React from 'react';
import Dialog from '../Dialog';
import LogoUniversal from '../../assets/images/logo-universal.png';

interface AboutDialogProps {
    show: boolean;
    onClose: () => void;
    licenseEmail: string | null;
    licenseEndDate: Date | null;
}

const AboutDialog: React.FC<AboutDialogProps> = ({ show, onClose, licenseEmail, licenseEndDate }) => {
    return (
        <Dialog show={show} onClose={onClose} title="About" maxWidth={500}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 20, padding: '24px 20px' }}>
                <img src={LogoUniversal} alt="BreachLine logo" style={{ maxWidth: 200, width: '70%', height: 'auto' }} />

                <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: 20, fontWeight: 500 }}>BreachLine</div>
                    <div style={{ fontSize: 14, opacity: 0.7, marginBottom: 10 }}>v0.0.13</div>
                    <div style={{ fontSize: 14 }}>https://breachline.app/</div>
                    <div style={{ fontSize: 14, opacity: 0.7, marginTop: 16 }}>
                        {licenseEmail ? `Licensed to ${licenseEmail}` : 'UNLICENSED'}
                    </div>
                    {licenseEndDate && (() => {
                        const now = new Date();
                        const diffMs = licenseEndDate.getTime() - now.getTime();
                        const daysUntilExpiry = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
                        return (
                            <div style={{ fontSize: 14, opacity: 0.7, marginTop: 4 }}>
                                Expires in {daysUntilExpiry} {daysUntilExpiry === 1 ? 'day' : 'days'}
                            </div>
                        );
                    })()}
                </div>
            </div>
        </Dialog>
    );
};

export default AboutDialog;
