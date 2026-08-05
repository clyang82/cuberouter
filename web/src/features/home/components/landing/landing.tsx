/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/*
 * CubeRouter landing page — TS port of the searouter-isuanova home page.
 * Renders hero (copy + WebGL globe + terminal + stats), features, enterprise,
 * quick-start steps, CTA, and a landing footer. The site header comes from
 * PublicLayout; this component owns everything below it.
 */

import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import Globe from './globe'
import './landing.css'

// Marketing demo base URL: shows the brand domain instead of the deployment host.
const DEMO_API_BASE = 'https://cube-router.com'

// Syntax-highlighted API call examples (markup matches landing.css .c-* classes).
const CODE_EXAMPLES: Record<string, string> = {
  curl: `<span class="c-prompt">$</span> curl ${DEMO_API_BASE}/v1/chat/completions <span class="c-punct">\\</span>
    -H <span class="c-str">"Authorization: Bearer </span><span class="c-var">$CR_API_KEY</span><span class="c-str">"</span> <span class="c-punct">\\</span>
    -H <span class="c-str">"Content-Type: application/json"</span> <span class="c-punct">\\</span>
    -d <span class="c-str">'{"model":"claude-sonnet-4","messages":[{"role":"user","content":"Hello"}],"stream":true}'</span>`,

  python: `<span class="c-key">from</span> openai <span class="c-key">import</span> OpenAI

client = OpenAI(
    base_url=<span class="c-str">"${DEMO_API_BASE}/v1"</span>,
    api_key=<span class="c-var">"$CR_API_KEY"</span>,
)

stream = client.chat.completions.create(
    model=<span class="c-str">"claude-sonnet-4"</span>,
    messages=[{<span class="c-str">"role"</span>: <span class="c-str">"user"</span>, <span class="c-str">"content"</span>: <span class="c-str">"Hello"</span>}],
    stream=<span class="c-key">True</span>,
)
<span class="c-key">for</span> chunk <span class="c-key">in</span> stream:
    <span class="c-key">print</span>(chunk.choices[<span class="c-num">0</span>].delta.content <span class="c-key">or</span> <span class="c-str">""</span>, end=<span class="c-str">""</span>)`,

  node: `<span class="c-key">import</span> OpenAI <span class="c-key">from</span> <span class="c-str">"openai"</span>;

<span class="c-key">const</span> client = <span class="c-key">new</span> OpenAI({
  baseURL: <span class="c-str">"${DEMO_API_BASE}/v1"</span>,
  apiKey: process.env.<span class="c-var">CR_API_KEY</span>,
});

<span class="c-key">const</span> stream = <span class="c-key">await</span> client.chat.completions.create({
  model: <span class="c-str">"claude-sonnet-4"</span>,
  messages: [{ role: <span class="c-str">"user"</span>, content: <span class="c-str">"Hello"</span> }],
  stream: <span class="c-key">true</span>,
});

<span class="c-key">for await</span> (<span class="c-key">const</span> chunk <span class="c-key">of</span> stream) {
  process.stdout.write(chunk.choices[<span class="c-num">0</span>]?.delta?.content ?? <span class="c-str">""</span>);
}`,
}

const TABS = [
  { id: 'curl', label: 'curl' },
  { id: 'python', label: 'python' },
  { id: 'node', label: 'node' },
]

// Inline SVG icons (lucide style, stroke 1.6).
const IconRoute = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M21 12a9 9 0 1 1-3.15-6.85M21 4v5h-5' />
  </svg>
)
const IconBolt = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M13 2 3 14h8l-1 8 10-12h-8l1-8z' />
  </svg>
)
const IconShield = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z' />
  </svg>
)
const IconChart = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M3 3v18h18M7 14l4-4 4 4 5-5' />
  </svg>
)

export function Landing() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('curl')

  return (
    <div className='landing-cuberouter'>
      {/* ==================== Hero ==================== */}
      <section className='cr-hero' id='top'>
        <div className='cr-container cr-hero__inner'>
          <div className='cr-hero__grid'>
            <div className='cr-hero__copy'>
              <span className='cr-hero__badge'>
                <span className='cr-hero__badge-dot' aria-hidden='true' />
                {t('landing.hero.badge')}
              </span>

              <h1 className='cr-hero__title'>
                <span className='cr-hero__title-prefix'>The AI</span>
                <span className='cr-hero__title-brand'>
                  CubeRouter<span className='cr-hero__title-dim'>.</span>
                </span>
              </h1>

              <p className='cr-hero__subtitle'>{t('landing.hero.subtitle')}</p>

              <div className='cr-hero__actions'>
                <Link to='/sign-up' className='cr-btn cr-btn--primary cr-btn--lg'>
                  {t('landing.hero.cta1')}
                </Link>
                <Link to='/pricing' className='cr-btn cr-btn--ghost cr-btn--lg'>
                  {t('landing.hero.cta2')}
                </Link>
              </div>
            </div>

            <div className='cr-hero__visual'>
              <Globe />
            </div>
          </div>

          <div
            className='cr-terminal'
            role='region'
            aria-label={t('landing.aria.apiExample')}
          >
            <div className='cr-terminal__bar'>
              <span className='cr-terminal__dot' aria-hidden='true' />
              <span className='cr-terminal__dot' aria-hidden='true' />
              <span className='cr-terminal__dot' aria-hidden='true' />
              <div className='cr-terminal__tabs' role='tablist'>
                {TABS.map(({ id, label }) => (
                  <button
                    key={id}
                    type='button'
                    role='tab'
                    aria-selected={activeTab === id}
                    className={`cr-terminal__tab ${activeTab === id ? 'cr-terminal__tab--active' : ''}`}
                    onClick={() => setActiveTab(id)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
            <div className='cr-terminal__body'>
              <pre dangerouslySetInnerHTML={{ __html: CODE_EXAMPLES[activeTab] }} />
            </div>
          </div>
        </div>

        <div className='cr-stats'>
          <div className='cr-container cr-stats__inner'>
            <div className='cr-stat'>
              <div className='cr-stat__value'>30T+</div>
              <div className='cr-stat__label'>{t('landing.hero.statToken')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>5M+</div>
              <div className='cr-stat__label'>{t('landing.hero.statUsers')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>60+</div>
              <div className='cr-stat__label'>{t('landing.hero.statProviders')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>300+</div>
              <div className='cr-stat__label'>{t('landing.hero.statModels')}</div>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== Features ==================== */}
      <section className='cr-section cr-section--no-border' id='features'>
        <div className='cr-container'>
          <div className='cr-section__head'>
            <div className='cr-section__eyebrow'>{t('landing.features.eyebrow')}</div>
            <h2 className='cr-section__title'>{t('landing.features.title')}</h2>
            <p className='cr-section__desc'>{t('landing.features.desc')}</p>
          </div>

          <div className='cr-features'>
            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconRoute />
              </div>
              <h3 className='cr-feature__title'>{t('landing.features.relay.title')}</h3>
              <p className='cr-feature__desc'>{t('landing.features.relay.desc')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconBolt />
              </div>
              <h3 className='cr-feature__title'>{t('landing.features.perf.title')}</h3>
              <p className='cr-feature__desc'>{t('landing.features.perf.desc')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconShield />
              </div>
              <h3 className='cr-feature__title'>{t('landing.features.security.title')}</h3>
              <p className='cr-feature__desc'>{t('landing.features.security.desc')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconChart />
              </div>
              <h3 className='cr-feature__title'>{t('landing.features.control.title')}</h3>
              <p className='cr-feature__desc'>{t('landing.features.control.desc')}</p>
            </article>
          </div>
        </div>
      </section>

      {/* ==================== Enterprise ==================== */}
      <section className='cr-section' id='enterprise'>
        <div className='cr-container'>
          <div className='cr-enterprise'>
            <div>
              <div className='cr-section__eyebrow'>{t('landing.enterprise.eyebrow')}</div>
              <h2 className='cr-section__title' style={{ maxWidth: '12ch' }}>
                {t('landing.enterprise.title')}
              </h2>
              <p className='cr-section__desc'>{t('landing.enterprise.desc')}</p>
              <div className='cr-enterprise__actions'>
                <a
                  href='mailto:cube-router.sales@isuanova.com'
                  className='cr-btn cr-btn--primary'
                >
                  {t('landing.enterprise.contactSales')}
                </a>
                <Link to='/pricing' className='cr-btn cr-btn--ghost'>
                  {t('landing.nav.viewPricing')}
                </Link>
              </div>
            </div>

            <div className='cr-enterprise__items'>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>01</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('landing.enterprise.item1.title')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('landing.enterprise.item1.desc')}
                  </p>
                </div>
              </div>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>02</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('landing.enterprise.item2.title')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('landing.enterprise.item2.desc')}
                  </p>
                </div>
              </div>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>03</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('landing.enterprise.item3.title')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('landing.enterprise.item3.desc')}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== Quick Start ==================== */}
      <section className='cr-section cr-section--subtle'>
        <div className='cr-container'>
          <div className='cr-section__head'>
            <div className='cr-section__eyebrow'>{t('landing.quickstart.eyebrow')}</div>
            <h2 className='cr-section__title'>{t('landing.quickstart.title')}</h2>
            <p className='cr-section__desc'>{t('landing.quickstart.desc')}</p>
          </div>

          <div className='cr-steps'>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('landing.quickstart.step1.num')}</div>
              <h3 className='cr-step__title'>{t('landing.quickstart.step1.title')}</h3>
              <p className='cr-step__desc'>{t('landing.quickstart.step1.desc')}</p>
            </div>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('landing.quickstart.step2.num')}</div>
              <h3 className='cr-step__title'>{t('landing.quickstart.step2.title')}</h3>
              <p className='cr-step__desc'>{t('landing.quickstart.step2.desc')}</p>
            </div>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('landing.quickstart.step3.num')}</div>
              <h3 className='cr-step__title'>{t('landing.quickstart.step3.title')}</h3>
              <p className='cr-step__desc'>{t('landing.quickstart.step3.desc')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== CTA ==================== */}
      <section className='cr-section'>
        <div className='cr-container cr-cta__inner'>
          <div className='cr-section__eyebrow'>{t('landing.cta.eyebrow')}</div>
          <h2 className='cr-cta__title'>{t('landing.cta.title')}</h2>
          <p className='cr-cta__desc'>{t('landing.cta.desc')}</p>
          <div className='cr-cta__actions'>
            <Link to='/sign-up' className='cr-btn cr-btn--primary cr-btn--lg'>
              {t('landing.cta.btnTrial')}
            </Link>
            <a
              href='mailto:cube-router.cs-support@isuanova.com'
              className='cr-btn cr-btn--ghost cr-btn--lg'
            >
              {t('landing.cta.btnContact')}
            </a>
          </div>
        </div>
      </section>

      {/* ==================== Footer ==================== */}
      <footer className='cr-footer' id='contact'>
        <div className='cr-container'>
          <div className='cr-footer__grid'>
            <div>
              <div className='cr-footer__brand'>
                <img src='/head.png' alt='CubeRouter' />
              </div>
              <p className='cr-footer__desc'>{t('landing.footer.desc')}</p>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('landing.footer.product')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <a href='#features' className='cr-footer__link'>
                    {t('landing.footer.features')}
                  </a>
                </li>
                <li>
                  <Link to='/pricing' className='cr-footer__link'>
                    {t('landing.footer.modelList')}
                  </Link>
                </li>
                <li>
                  <Link to='/pricing' className='cr-footer__link'>
                    {t('landing.footer.pricing')}
                  </Link>
                </li>
              </ul>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('landing.footer.enterprise')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <a href='#enterprise' className='cr-footer__link'>
                    {t('landing.footer.enterpriseEdition')}
                  </a>
                </li>
                <li>
                  <a
                    href='mailto:cube-router.sales@isuanova.com'
                    className='cr-footer__link'
                  >
                    {t('landing.enterprise.contactSales')}
                  </a>
                </li>
              </ul>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('landing.footer.legal')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <Link to='/privacy-policy' className='cr-footer__link'>
                    {t('landing.footer.privacy')}
                  </Link>
                </li>
                <li>
                  <Link to='/user-agreement' className='cr-footer__link'>
                    {t('landing.footer.terms')}
                  </Link>
                </li>
              </ul>
            </div>
          </div>

          <div className='cr-footer__bottom'>
            <span>{t('landing.footer.copyright')}</span>
            <span>All Rights Reserved</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
