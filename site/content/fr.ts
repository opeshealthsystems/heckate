import type { Dict } from "./types";

// French copy mirrors the English structure. Needs a native review before launch (docs/BLOCKERS.md).
export const fr: Dict = {
  lang: "fr",
  locale: "fr_CM",
  nav: { home: "Accueil", services: "Services", about: "À propos", contact: "Contact", portal: "Espace client", verify: "Vérifier un document" },
  home: {
    title: "HEKATE Systems — cybersécurité, identité numérique et services TIC au Cameroun",
    description: "HEKATE Systems propose des tests d'intrusion, des évaluations de sécurité, la vérification d'identité numérique et la réponse aux incidents depuis Douala, Cameroun.",
    h1: "Cybersécurité et identité numérique, conçues au Cameroun",
    lead: "Nous testons vos défenses, vérifions les identités et intervenons en cas de problème, avec un espace client qui regroupe tickets, documents et certificats.",
    cta: "Nous contacter",
    ctaSupport: "Signaler un incident",
    servicesH: "Nos services",
    platformH: "Une seule plateforme pour votre support et vos documents",
    platform: [
      "Signalez un incident depuis l'espace client ou l'application mobile et suivez-le en direct.",
      "Chaque ticket porte un minuteur de niveau de service : vous voyez quand nous devons vous répondre.",
      "Devis, factures, contrats et rapports sont disponibles dans votre espace client.",
      "Chaque document émis porte une référence et une page de vérification publique.",
    ],
    trustH: "Confiance et sécurité",
    trust: [
      "Connexion à deux facteurs pour tout le personnel et accès par rôle aux données clients.",
      "Chaque client ne voit que ses propres données, garanti au niveau de la base de données.",
      "Stockage et sauvegardes chiffrés, avec une piste d'audit de chaque modification.",
      "Notre plateforme est soumise à des tests d'intrusion avant sa mise en service.",
    ],
    supportH: "Besoin d'aide maintenant ?",
    support: "Les clients peuvent ouvrir un ticket dans l'espace client ou l'application. En cas d'urgence, écrivez à l'adresse incident ci-dessous : nous ouvrirons un ticket P1.",
  },
  services: {
    title: "Services — HEKATE Systems",
    description: "Tests d'intrusion, évaluation de sécurité, vérification d'identité numérique hecID, vérification des antécédents et réponse aux incidents.",
    h1: "Services",
    items: [
      {
        slug: "penetration-testing",
        name: "Tests d'intrusion",
        summary: "Attaques autorisées et cadrées sur vos applications, réseaux et infrastructures, avec un rapport écrit et un nouveau test.",
        faq: [
          { q: "Combien coûte un test d'intrusion au Cameroun ?", a: "Le prix dépend du périmètre : nombre d'applications, de réseaux et d'environnements. Nous cadrons d'abord la mission puis envoyons un devis ferme avant tout travail. Utilisez le formulaire de contact pour en demander un." },
          { q: "Que recevons-nous à la fin ?", a: "Un rapport signé et confidentiel, avec les constats classés par gravité et les corrections à appliquer. Un nouveau test des corrections est suivi pour la mission." },
          { q: "Faut-il notre autorisation écrite ?", a: "Oui. Chaque test commence par une autorisation signée et des règles d'engagement." },
        ],
      },
      {
        slug: "security-assessment",
        name: "Évaluation de sécurité",
        summary: "Revue structurée de vos mesures, politiques et configurations au regard des bonnes pratiques reconnues.",
        faq: [
          { q: "En quoi une évaluation diffère-t-elle d'un test d'intrusion ?", a: "L'évaluation examine votre organisation : politiques, configuration et processus. Le test d'intrusion attaque les systèmes pour prouver ce qui est exploitable. Beaucoup de clients font les deux." },
          { q: "Pouvez-vous délivrer un certificat ?", a: "Lorsqu'une évaluation répond aux critères convenus à l'avance, nous délivrons un certificat dont la référence peut être vérifiée par tous sur notre page de vérification." },
        ],
      },
      {
        slug: "digital-identity",
        name: "Identité numérique (hecID) et e-KYC",
        summary: "Vérification d'identité et intégration e-KYC pour savoir qui sont vos clients.",
        faq: [
          { q: "Qu'est-ce que hecID ?", a: "hecID est le service d'identité numérique de HEKATE. Il permet aux organisations de vérifier électroniquement l'identité d'une personne lors de l'intégration." },
          { q: "Comment l'intégrer ?", a: "Envoyez une demande d'intégration e-KYC via le formulaire de contact : nous définirons le périmètre et les étapes techniques avec votre équipe." },
        ],
      },
      {
        slug: "background-screening",
        name: "Vérification des antécédents",
        summary: "Vérifications fondées sur le consentement pour le recrutement et la diligence partenaires, traitées comme données restreintes.",
        faq: [
          { q: "Faut-il le consentement de la personne ?", a: "Oui. La vérification ne commence qu'avec un consentement signé, et les résultats sont traités comme des informations restreintes." },
        ],
      },
      {
        slug: "incident-response",
        name: "Réponse aux incidents",
        summary: "Aide d'urgence en cas d'attaque ou de compromission constatée, avec traçabilité de chaque action.",
        faq: [
          { q: "Comment signaler un incident ?", a: "Les clients ouvrent un ticket dans l'espace client ou l'application et le marquent P1 si le service est à l'arrêt. Vous pouvez aussi écrire à l'adresse incident ; chaque canal devient un seul ticket." },
          { q: "Qui répond à un incident critique ?", a: "Un P1 alerte en même temps l'ingénieur d'astreinte et un responsable, et le chronomètre du niveau de service démarre immédiatement." },
        ],
      },
    ],
  },
  about: {
    title: "À propos — HEKATE Systems",
    description: "HEKATE SYSTEMS SARL est une société de cybersécurité et d'identité numérique basée à Douala, Cameroun.",
    h1: "À propos de HEKATE",
    body: [
      "HEKATE SYSTEMS SARL est une société de cybersécurité et d'identité numérique basée à Douala, Cameroun.",
      "Nous aidons les organisations à tester leurs défenses, vérifier les identités et répondre aux incidents, et nous exploitons notre propre plateforme client pour suivre au même endroit le support, les documents et les certificats.",
    ],
  },
  contact: {
    title: "Contact — HEKATE Systems",
    description: "Contactez HEKATE Systems à Douala, Cameroun, ou signalez un incident.",
    h1: "Nous contacter",
    intro: "Dites-nous ce dont vous avez besoin et nous vous répondrons par e-mail.",
    emergencyH: "Incident ou urgence",
    emergency: "Clients : ouvrez un ticket P1 dans l'espace client ou l'application. Sinon, écrivez à",
    name: "Votre nom", email: "E-mail", phone: "Téléphone (facultatif)", company: "Société (facultatif)", message: "Comment pouvons-nous vous aider ?",
    send: "Envoyer", sending: "Envoi…",
    thanks: "Merci. Nous avons bien reçu votre message et vous répondrons par e-mail.",
    error: "Désolé, votre message n'a pas pu être envoyé. Écrivez-nous par e-mail.",
    note: "Nous utilisons vos coordonnées uniquement pour vous répondre.",
  },
  footer: { verify: "Vérifier un document HEKATE", rights: "Tous droits réservés." },
};
