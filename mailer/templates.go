package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// BaseLayoutData represents the common wrapper context for email rendering.
type BaseLayoutData struct {
	AppName      string
	Preheader    string
	Subject      string
	BadgeText    string
	BadgeColor   string // e.g. indigo, emerald, amber, sky
	HeaderTitle  string
	ContentHTML  template.HTML
	SupportEmail string
	PortalURL    string
	CurrentYear  int
	SystemNotice string
}

// SetupEmailData holds data needed to render an account activation email.
type SetupEmailData struct {
	AppName      string
	Username     string
	Email        string
	SetupURL     string
	ExpireDays   int
	ExpiresAt    string
	SupportEmail string
	PortalURL    string
}

// ResetEmailData holds data needed to render a password reset email.
type ResetEmailData struct {
	AppName       string
	Username      string
	Email         string
	ResetURL      string
	ExpireMinutes int
	ExpiresAt     string
	SupportEmail  string
	PortalURL     string
	RequestedAt   string
}

// TimesheetEmailData holds data needed to render a timesheet delivery email.
type TimesheetEmailData struct {
	AppName      string
	Username     string
	Email        string
	Company      string
	Period       string
	Filename     string
	PortalURL    string
	SupportEmail string
}

// ReminderEmailData holds data needed to render a daily timesheet reminder email.
type ReminderEmailData struct {
	AppName      string
	Username     string
	Email        string
	Date         string
	ActivityURL  string
	PortalURL    string
	SupportEmail string
}

// PasswordChangedEmailData holds data needed to render a password change notification email.
type PasswordChangedEmailData struct {
	AppName      string
	Username     string
	Email        string
	ChangedAt    string
	LoginURL     string
	SupportEmail string
	PortalURL    string
}

// Shared CSS styles for high-fidelity cross-client email rendering.
const baseLayoutHTML = `<!DOCTYPE html>
<html lang="id" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>{{.Subject}}</title>
  <!--[if mso]>
  <style type="text/css">
    body, table, td, a { font-family: Arial, Helvetica, sans-serif !important; }
  </style>
  <![endif]-->
  <style type="text/css">
    body {
      margin: 0;
      padding: 0;
      background-color: #f1f5f9;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
      color: #334155;
      line-height: 1.6;
    }
    table {
      border-collapse: collapse;
      mso-table-lspace: 0pt;
      mso-table-rspace: 0pt;
    }
    td {
      padding: 0;
    }
    img {
      border: 0;
      height: auto;
      line-height: 100%;
      outline: none;
      text-decoration: none;
    }
    .email-container {
      max-width: 600px;
      margin: 0 auto;
      width: 100%;
    }
    .btn-primary {
      background: linear-gradient(135deg, #4f46e5 0%, #4338ca 100%);
      background-color: #4f46e5;
      color: #ffffff !important;
      padding: 14px 28px;
      text-decoration: none;
      border-radius: 8px;
      font-weight: 600;
      font-size: 15px;
      display: inline-block;
      text-align: center;
      letter-spacing: 0.2px;
      box-shadow: 0 4px 10px rgba(79, 70, 229, 0.25);
    }
    .btn-primary:hover {
      background-color: #4338ca !important;
    }
    @media only screen and (max-width: 600px) {
      .responsive-card {
        padding: 24px 18px !important;
        border-radius: 8px !important;
      }
      .mobile-center {
        text-align: center !important;
      }
      .mobile-btn {
        display: block !important;
        width: 100% !important;
        box-sizing: border-box !important;
      }
      .mobile-stack {
        display: block !important;
        width: 100% !important;
      }
    }
  </style>
</head>
<body style="margin: 0; padding: 0; background-color: #f1f5f9; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #334155; line-height: 1.6;">

  {{if .Preheader}}
  <!-- Preheader text (hidden preview in inbox) -->
  <div style="display: none; font-size: 1px; color: #f1f5f9; line-height: 1px; max-height: 0px; max-width: 0px; opacity: 0; overflow: hidden;">
    {{.Preheader}}
  </div>
  {{end}}

  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f1f5f9; width: 100%; padding: 32px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" class="email-container" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width: 600px; margin: 0 auto;">
          
          <!-- MAIN CARD -->
          <tr>
            <td style="background-color: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; box-shadow: 0 8px 24px rgba(15, 23, 42, 0.05); overflow: hidden;">
              
              <!-- TOP ACCENT COLOR BAR -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td height="5" style="background: linear-gradient(90deg, #4f46e5 0%, #3b82f6 50%, #06b6d4 100%); font-size: 0; line-height: 0;">&nbsp;</td>
                </tr>
              </table>

              <!-- CARD CONTENT CONTAINER -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td class="responsive-card" style="padding: 36px 36px 32px 36px;">
                    
                    {{if .BadgeText}}
                    <!-- CATEGORY / STATUS BADGE -->
                    <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 20px;">
                      <tr>
                        {{if eq .BadgeColor "emerald"}}
                        <td style="background-color: #ecfdf5; border: 1px solid #a7f3d0; padding: 5px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #047857; text-transform: uppercase; letter-spacing: 0.5px;">
                          ● {{.BadgeText}}
                        </td>
                        {{else if eq .BadgeColor "amber"}}
                        <td style="background-color: #fffbeb; border: 1px solid #fde68a; padding: 5px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #b45309; text-transform: uppercase; letter-spacing: 0.5px;">
                          ● {{.BadgeText}}
                        </td>
                        {{else if eq .BadgeColor "sky"}}
                        <td style="background-color: #f0f9ff; border: 1px solid #bae6fd; padding: 5px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #0284c7; text-transform: uppercase; letter-spacing: 0.5px;">
                          ● {{.BadgeText}}
                        </td>
                        {{else}}
                        <td style="background-color: #eef2ff; border: 1px solid #c7d2fe; padding: 5px 12px; border-radius: 20px; font-size: 12px; font-weight: 600; color: #4338ca; text-transform: uppercase; letter-spacing: 0.5px;">
                          ● {{.BadgeText}}
                        </td>
                        {{end}}
                      </tr>
                    </table>
                    {{end}}

                    {{if .HeaderTitle}}
                    <h1 style="margin: 0 0 16px 0; font-size: 22px; font-weight: 700; color: #0f172a; line-height: 1.35; letter-spacing: -0.4px;">
                      {{.HeaderTitle}}
                    </h1>
                    {{end}}

                    <!-- DYNAMIC INJECTED CONTENT -->
                    {{.ContentHTML}}

                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- FOOTER -->
          <tr>
            <td style="padding: 28px 16px 16px 16px; text-align: center;">
              <p style="margin: 0 0 8px 0; font-size: 12px; color: #64748b; line-height: 1.5;">
                {{if .SystemNotice}}{{.SystemNotice}}{{else}}Email ini dikirimkan secara otomatis oleh sistem <strong>{{.AppName}}</strong>. Mohon untuk tidak membalas email ini.{{end}}
              </p>
              {{if .SupportEmail}}
              <p style="margin: 0 0 12px 0; font-size: 12px; color: #64748b;">
                Bantuan & pertanyaan: <a href="mailto:{{.SupportEmail}}" style="color: #4f46e5; text-decoration: underline;">{{.SupportEmail}}</a>
              </p>
              {{end}}
              <div style="height: 1px; background-color: #e2e8f0; margin: 16px auto; max-width: 200px;"></div>
              <p style="margin: 0; font-size: 11px; color: #94a3b8;">
                &copy; {{.CurrentYear}} {{.AppName}}. All rights reserved.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>

</body>
</html>`

// --- 1. SETUP / ACTIVATION EMAIL ---

const setupContentTemplate = `
<p style="margin: 0 0 16px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Halo <strong>{{.Username}}</strong>,
</p>
<p style="margin: 0 0 20px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Administrator telah membuat akun baru untuk Anda pada <strong>{{.AppName}}</strong>. Silakan selesaikan aktivasi akun dengan mengatur kata sandi Anda melalui tautan di bawah ini.
</p>

<!-- DETAILS BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Username</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #0f172a;">{{.Username}}</td>
        </tr>
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Berlaku Hingga</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 500; color: #0f172a;">{{if .ExpiresAt}}{{.ExpiresAt}}{{else}}{{.ExpireDays}} hari{{end}}</td>
        </tr>
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Status Akun</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #d97706;">Menunggu Aktivasi</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- ACTION BUTTON -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 24px;">
  <tr>
    <td align="center">
      <a href="{{.SetupURL}}" target="_blank" class="btn-primary mobile-btn">
        Aktivasi Akun &amp; Atur Password &rarr;
      </a>
    </td>
  </tr>
</table>

<!-- FALLBACK URL -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px dashed #cbd5e1; border-radius: 8px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 12px 16px;">
      <p style="margin: 0 0 6px 0; font-size: 12px; color: #64748b; font-weight: 600;">
        Jika tombol di atas tidak dapat diklik, salin dan tempel URL berikut ke browser Anda:
      </p>
      <p style="margin: 0; font-size: 12px; word-break: break-all; color: #4f46e5; font-family: monospace;">
        <a href="{{.SetupURL}}" target="_blank" style="color: #4f46e5; text-decoration: underline;">{{.SetupURL}}</a>
      </p>
    </td>
  </tr>
</table>

<!-- FEATURE HIGHLIGHT / WEBAUTHN NOTICE -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #eff6ff; border-left: 4px solid #3b82f6; border-radius: 0 8px 8px 0; margin-bottom: 16px;">
  <tr>
    <td style="padding: 14px 16px;">
      <p style="margin: 0 0 4px 0; font-size: 13px; font-weight: 700; color: #1e40af;">
        💡 Dukungan Login Biometrik / Passkey
      </p>
      <p style="margin: 0; font-size: 12px; color: #1e3a8a; line-height: 1.5;">
        Setelah mengatur kata sandi, Anda dapat mendaftarkan Passkey (Face ID, Touch ID, atau Windows Hello) pada menu Profil untuk proses masuk yang instan dan tanpa kata sandi.
      </p>
    </td>
  </tr>
</table>

<p style="margin: 16px 0 0 0; font-size: 12px; color: #94a3b8; line-height: 1.5;">
  Jika Anda tidak merasa meminta pembuatan akun ini, Anda dapat mengabaikan email ini dengan aman.
</p>
`

// --- 2. PASSWORD RESET EMAIL ---

const resetContentTemplate = `
<p style="margin: 0 0 16px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Halo{{if .Username}} <strong>{{.Username}}</strong>{{end}},
</p>
<p style="margin: 0 0 20px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Kami menerima permintaan untuk mengatur ulang kata sandi akun <strong>{{.AppName}}</strong> yang terhubung dengan alamat email ini.
</p>

<!-- DETAILS BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
        {{if .Username}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Akun</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #0f172a;">{{.Username}}</td>
        </tr>
        {{end}}
        {{if .RequestedAt}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Waktu Permintaan</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-weight: 500; color: #0f172a;">{{.RequestedAt}}</td>
        </tr>
        {{end}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Berlaku Hingga</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 600; color: #dc2626;">{{if .ExpiresAt}}{{.ExpiresAt}}{{else}}{{.ExpireMinutes}} menit{{end}}</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- ACTION BUTTON -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 24px;">
  <tr>
    <td align="center">
      <a href="{{.ResetURL}}" target="_blank" class="btn-primary mobile-btn">
        Reset Kata Sandi Saya &rarr;
      </a>
    </td>
  </tr>
</table>

<!-- FALLBACK URL -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px dashed #cbd5e1; border-radius: 8px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 12px 16px;">
      <p style="margin: 0 0 6px 0; font-size: 12px; color: #64748b; font-weight: 600;">
        Jika tombol di atas tidak dapat diklik, salin dan tempel URL berikut ke browser Anda:
      </p>
      <p style="margin: 0; font-size: 12px; word-break: break-all; color: #4f46e5; font-family: monospace;">
        <a href="{{.ResetURL}}" target="_blank" style="color: #4f46e5; text-decoration: underline;">{{.ResetURL}}</a>
      </p>
    </td>
  </tr>
</table>

<!-- SECURITY ALERT NOTICE -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #fffbeb; border-left: 4px solid #f59e0b; border-radius: 0 8px 8px 0; margin-bottom: 16px;">
  <tr>
    <td style="padding: 14px 16px;">
      <p style="margin: 0 0 4px 0; font-size: 13px; font-weight: 700; color: #92400e;">
        🔒 Peringatan Keamanan
      </p>
      <p style="margin: 0; font-size: 12px; color: #78350f; line-height: 1.5;">
        Tautan ini bersifat rahasia dan hanya berlaku hingga <strong>{{if .ExpiresAt}}{{.ExpiresAt}}{{else}}{{.ExpireMinutes}} menit{{end}}</strong>. Jangan bagikan tautan ini kepada siapapun. Jika Anda tidak pernah meminta reset kata sandi, abaikan email ini; kata sandi Anda tetap aman dan tidak berubah.
      </p>
    </td>
  </tr>
</table>
`

// --- 3. TIMESHEET DELIVERY EMAIL ---

const timesheetContentTemplate = `
<p style="margin: 0 0 16px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Halo{{if .Username}} <strong>{{.Username}}</strong>{{end}},
</p>
<p style="margin: 0 0 20px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Dokumen timesheet bulanan Anda telah berhasil dibuat dan <strong>terlampir</strong> pada email ini dalam format Microsoft Excel (.xlsx). Salinan dokumen ini juga telah otomatis diunduh pada browser Anda.
</p>

<!-- DETAILS BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
        {{if .Company}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Perusahaan / Klien</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #0f172a;">{{.Company}}</td>
        </tr>
        {{end}}
        {{if .Period}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Periode Timesheet</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #4f46e5;">{{.Period}}</td>
        </tr>
        {{end}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Nama Lampiran</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-family: monospace; font-weight: 600; color: #0f172a;">{{.Filename}}</td>
        </tr>
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Format Berkas</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-weight: 500; color: #059669;">Microsoft Excel Spreadsheet (.xlsx)</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- NEXT STEPS / ADVICE BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #ecfdf5; border-left: 4px solid #10b981; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
  <tr>
    <td style="padding: 14px 16px;">
      <p style="margin: 0 0 4px 0; font-size: 13px; font-weight: 700; color: #065f46;">
        📌 Langkah Selanjutnya (Approval)
      </p>
      <p style="margin: 0; font-size: 12px; color: #047857; line-height: 1.5;">
        Silakan periksa kembali rincian hari kerja, jam lembur, dan uraian tugas pada file lampiran. Setelah diverifikasi, serahkan dokumen kepada Team Leader atau Department Head Anda untuk proses persetujuan dan tanda tangan resmi.
      </p>
    </td>
  </tr>
</table>

{{if .PortalURL}}
<!-- ACTION BUTTON -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 16px;">
  <tr>
    <td align="center">
      <a href="{{.PortalURL}}" target="_blank" class="btn-primary mobile-btn">
        Buka Timesheet Portal &rarr;
      </a>
    </td>
  </tr>
</table>
{{end}}
`

// --- 4. DAILY REMINDER EMAIL ---

const reminderContentTemplate = `
<p style="margin: 0 0 16px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Halo <strong>{{.Username}}</strong>,
</p>
<p style="margin: 0 0 20px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Ini adalah pengingat harian otomatis dari <strong>{{.AppName}}</strong>. Kami melihat Anda belum mencatat aktivitas kerja untuk hari ini.
</p>

<!-- DETAILS BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Tanggal</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #0f172a;">{{.Date}}</td>
        </tr>
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Status Log</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-weight: 700; color: #e11d48;">Belum Diisi</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

<!-- ACTION BUTTON -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 24px;">
  <tr>
    <td align="center">
      <a href="{{.ActivityURL}}" target="_blank" class="btn-primary mobile-btn">
        Isi Aktivitas Hari Ini &rarr;
      </a>
    </td>
  </tr>
</table>

<!-- TIP BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f0f9ff; border-left: 4px solid #0ea5e9; border-radius: 0 8px 8px 0; margin-bottom: 16px;">
  <tr>
    <td style="padding: 14px 16px;">
      <p style="margin: 0 0 4px 0; font-size: 13px; font-weight: 700; color: #0369a1;">
        ✨ Tips Pencatatan
      </p>
      <p style="margin: 0; font-size: 12px; color: #0c4a6e; line-height: 1.5;">
        Mencatat aktivitas kerja di akhir hari membantu menjaga akurasi laporan dan mempermudah rekapitulasi saat batas waktu akhir bulan (cutoff) tiba.
      </p>
    </td>
  </tr>
</table>
`

// --- 5. PASSWORD CHANGED NOTIFICATION EMAIL ---

const passwordChangedContentTemplate = `
<p style="margin: 0 0 16px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Halo{{if .Username}} <strong>{{.Username}}</strong>{{end}},
</p>
<p style="margin: 0 0 20px 0; font-size: 15px; color: #334155; line-height: 1.6;">
  Kata sandi untuk akun <strong>{{.AppName}}</strong> Anda telah berhasil diperbarui.
</p>

<!-- DETAILS BOX -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 10px; margin-bottom: 24px;">
  <tr>
    <td style="padding: 16px 20px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
        {{if .Username}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Akun</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 700; color: #0f172a;">{{.Username}}</td>
        </tr>
        {{end}}
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Waktu Perubahan</td>
          <td width="65%" style="padding: 6px 0; font-size: 14px; font-weight: 600; color: #0f172a;">{{.ChangedAt}}</td>
        </tr>
        <tr>
          <td width="35%" style="padding: 6px 0; font-size: 13px; font-weight: 600; color: #64748b; vertical-align: top;">Status Keamanan</td>
          <td width="65%" style="padding: 6px 0; font-size: 13px; font-weight: 700; color: #059669;">Kata Sandi Aktif</td>
        </tr>
      </table>
    </td>
  </tr>
</table>

{{if .LoginURL}}
<!-- ACTION BUTTON -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-bottom: 24px;">
  <tr>
    <td align="center">
      <a href="{{.LoginURL}}" target="_blank" class="btn-primary mobile-btn">
        Masuk ke Portal &rarr;
      </a>
    </td>
  </tr>
</table>
{{end}}

<!-- SECURITY ADVICE -->
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color: #fef2f2; border-left: 4px solid #ef4444; border-radius: 0 8px 8px 0; margin-bottom: 16px;">
  <tr>
    <td style="padding: 14px 16px;">
      <p style="margin: 0 0 4px 0; font-size: 13px; font-weight: 700; color: #991b1b;">
        ⚠️ Bukan Anda yang Melakukan Perubahan Ini?
      </p>
      <p style="margin: 0; font-size: 12px; color: #7f1d1d; line-height: 1.5;">
        Jika Anda tidak merasa mengubah kata sandi akun Anda, segera hubungi administrator sistem atau lakukan reset kata sandi darurat untuk mengamankan akun Anda.
      </p>
    </td>
  </tr>
</table>
`

// Parse parsed templates once for efficiency and thread safety.
var (
	baseTmpl            = template.Must(template.New("base").Parse(baseLayoutHTML))
	setupTmpl           = template.Must(template.New("setup").Parse(setupContentTemplate))
	resetTmpl           = template.Must(template.New("reset").Parse(resetContentTemplate))
	timesheetTmpl       = template.Must(template.New("timesheet").Parse(timesheetContentTemplate))
	reminderTmpl        = template.Must(template.New("reminder").Parse(reminderContentTemplate))
	passwordChangedTmpl = template.Must(template.New("pwd_changed").Parse(passwordChangedContentTemplate))
)

func renderWithLayout(layout BaseLayoutData, contentTmpl *template.Template, data any) (string, error) {
	var contentBuf bytes.Buffer
	if err := contentTmpl.Execute(&contentBuf, data); err != nil {
		return "", fmt.Errorf("failed to render content template: %w", err)
	}

	layout.ContentHTML = template.HTML(contentBuf.String())
	if layout.CurrentYear <= 0 {
		layout.CurrentYear = time.Now().Year()
	}
	if layout.AppName == "" {
		layout.AppName = "Timesheet Portal"
	}

	var finalBuf bytes.Buffer
	if err := baseTmpl.Execute(&finalBuf, layout); err != nil {
		return "", fmt.Errorf("failed to render base email layout: %w", err)
	}

	return finalBuf.String(), nil
}

// RenderSetupEmail generates both HTML and plain-text activation email content.
func RenderSetupEmail(data SetupEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ExpireDays <= 0 {
		data.ExpireDays = 7
	}
	if data.ExpiresAt == "" {
		data.ExpiresAt = time.Now().AddDate(0, 0, data.ExpireDays).Format("02 Jan 2006, 15:04 WIB")
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Aktivasi akun %s Anda dan atur kata sandi baru.", data.AppName),
		Subject:      fmt.Sprintf("Aktivasi Akun %s - Selamat Datang!", data.AppName),
		BadgeText:    "Aktivasi Akun",
		BadgeColor:   "indigo",
		HeaderTitle:  "Selamat Datang di " + data.AppName,
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, setupTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Administrator telah membuat akun baru untuk Anda pada %s.
Silakan selesaikan aktivasi akun dengan mengatur kata sandi Anda melalui tautan berikut:

%s

Detail Akun:
- Username: %s
- Berlaku Hingga: %s (%d hari)

Setelah mengatur kata sandi, Anda juga dapat mendaftarkan Passkey (biometrik) pada menu Profil untuk login yang lebih cepat dan aman.

Jika Anda tidak merasa meminta akun ini, Anda dapat mengabaikan email ini.

--
%s
`, data.Username, data.AppName, data.SetupURL, data.Username, data.ExpiresAt, data.ExpireDays, data.AppName)

	return htmlBody, textBody, nil
}

// RenderResetEmail generates both HTML and plain-text password reset email content.
func RenderResetEmail(data ResetEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ExpireMinutes <= 0 {
		data.ExpireMinutes = 60
	}
	if data.RequestedAt == "" {
		data.RequestedAt = time.Now().Format("02 Jan 2006, 15:04 WIB")
	}
	if data.ExpiresAt == "" {
		data.ExpiresAt = time.Now().Add(time.Duration(data.ExpireMinutes) * time.Minute).Format("02 Jan 2006, 15:04 WIB")
	}

	greetingName := data.Username
	if greetingName == "" {
		greetingName = "Pengguna"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Instruksi pengaturan ulang kata sandi akun %s Anda.", data.AppName),
		Subject:      fmt.Sprintf("Reset Kata Sandi Akun %s", data.AppName),
		BadgeText:    "Keamanan Akun",
		BadgeColor:   "amber",
		HeaderTitle:  "Permintaan Reset Kata Sandi",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, resetTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Kami menerima permintaan untuk mengatur ulang kata sandi akun %s Anda.
Gunakan tautan di bawah ini untuk membuat kata sandi baru:

%s

Detail Permintaan:
- Waktu Permintaan: %s
- Berlaku Hingga: %s

PENTING: Tautan ini bersifat rahasia dan hanya berlaku hingga %s. Jangan bagikan tautan ini kepada siapapun.
Jika Anda tidak meminta pengaturan ulang kata sandi ini, abaikan email ini; akun Anda tetap aman.

--
%s
`, greetingName, data.AppName, data.ResetURL, data.RequestedAt, data.ExpiresAt, data.ExpiresAt, data.AppName)

	return htmlBody, textBody, nil
}

// RenderTimesheetEmail generates both HTML and plain-text timesheet delivery email content.
func RenderTimesheetEmail(data TimesheetEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}

	headerCompany := data.Company
	if headerCompany == "" {
		headerCompany = "Timesheet"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Dokumen Timesheet %s telah siap diunduh.", headerCompany),
		Subject:      fmt.Sprintf("Dokumen Timesheet %s Anda Telah Siap", headerCompany),
		BadgeText:    "Laporan Timesheet",
		BadgeColor:   "emerald",
		HeaderTitle:  fmt.Sprintf("Dokumen Timesheet %s Telah Siap", headerCompany),
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, timesheetTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Dokumen timesheet bulanan Anda telah berhasil dibuat dan dilampirkan pada email ini (format .xlsx).
Salinan dokumen ini juga telah otomatis diunduh pada peramban web Anda.

Detail Timesheet:
- Perusahaan: %s
- Periode: %s
- Nama File: %s

Langkah Selanjutnya:
Silakan periksa kembali rincian jam kerja dan kegiatan sebelum menyerahkan dokumen ini kepada Team Leader / Department Head untuk persetujuan (approval).

--
%s
%s
`, data.Username, data.Company, data.Period, data.Filename, data.AppName, data.PortalURL)

	return htmlBody, textBody, nil
}

// RenderReminderEmail generates both HTML and plain-text reminder email content.
func RenderReminderEmail(data ReminderEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.Date == "" {
		data.Date = time.Now().Format("02 January 2006")
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Pengingat: Waktunya mencatat aktivitas kerja hari ini (%s).", data.Date),
		Subject:      fmt.Sprintf("Pengingat Harian: Isi Timesheet Hari Ini (%s)", data.Date),
		BadgeText:    "Pengingat Harian",
		BadgeColor:   "sky",
		HeaderTitle:  "Pengingat Pengisian Timesheet",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, reminderTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Ini adalah pengingat harian otomatis dari %s.
Anda belum mencatat aktivitas kerja untuk hari ini (%s).

Silakan isi aktivitas kerja Anda melalui tautan berikut:
%s

Mencatat aktivitas kerja secara rutin setiap hari membantu menjaga keakuratan jam kerja dan kelancaran rekapitulasi di akhir bulan.

--
%s
`, data.Username, data.AppName, data.Date, data.ActivityURL, data.AppName)

	return htmlBody, textBody, nil
}

// RenderPasswordChangedEmail generates both HTML and plain-text password changed notification email content.
func RenderPasswordChangedEmail(data PasswordChangedEmailData) (htmlBody string, textBody string, err error) {
	if data.AppName == "" {
		data.AppName = "Timesheet Portal"
	}
	if data.ChangedAt == "" {
		data.ChangedAt = time.Now().Format("02 Jan 2006, 15:04 WIB")
	}

	greetingName := data.Username
	if greetingName == "" {
		greetingName = "Pengguna"
	}

	layout := BaseLayoutData{
		AppName:      data.AppName,
		Preheader:    fmt.Sprintf("Kata sandi akun %s Anda telah berhasil diperbarui.", data.AppName),
		Subject:      fmt.Sprintf("Keamanan Akun: Kata Sandi %s Telah Diperbarui", data.AppName),
		BadgeText:    "Pembaruan Keamanan",
		BadgeColor:   "emerald",
		HeaderTitle:  "Kata Sandi Berhasil Diperbarui",
		SupportEmail: data.SupportEmail,
		PortalURL:    data.PortalURL,
	}

	htmlBody, err = renderWithLayout(layout, passwordChangedTmpl, data)
	if err != nil {
		return "", "", err
	}

	textBody = fmt.Sprintf(`Halo %s,

Kata sandi akun %s Anda telah berhasil diperbarui pada %s.

Jika Anda yang melakukan perubahan ini, Anda dapat masuk kembali dengan kata sandi baru Anda:
%s

PERINGATAN KEAMANAN:
Jika Anda TIDAK pernah meminta atau melakukan perubahan ini, segera hubungi administrator sistem Anda untuk mengamankan akun Anda.

--
%s
`, greetingName, data.AppName, data.ChangedAt, data.LoginURL, data.AppName)

	return htmlBody, textBody, nil
}

// FormatMonthYearIndonesian converts numeric month and year to Indonesian string (e.g. 9, 2026 -> "September 2026").
func FormatMonthYearIndonesian(month int, year int) string {
	months := [...]string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %04d", months[month-1], year)
	}
	if year > 0 {
		return fmt.Sprintf("Bulan %02d %04d", month, year)
	}
	return ""
}

// ParsePeriodFromFilename attempts to extract month and year from a standard timesheet filename
// like "Timesheet_john_doe_09_2026.xlsx".
func ParsePeriodFromFilename(filename string) (string, string) {
	// e.g. Timesheet_john_doe_09_2026.xlsx -> remove extension and split by "_"
	base := strings.TrimSuffix(filename, ".xlsx")
	parts := strings.Split(base, "_")
	if len(parts) >= 3 {
		yearStr := parts[len(parts)-1]
		monthStr := parts[len(parts)-2]
		var m, y int
		if _, err := fmt.Sscanf(monthStr, "%d", &m); err == nil {
			if _, err := fmt.Sscanf(yearStr, "%d", &y); err == nil && y >= 2000 && y <= 2100 {
				return FormatMonthYearIndonesian(m, y), strings.Join(parts[1:len(parts)-2], "_")
			}
		}
	}
	return "", ""
}
