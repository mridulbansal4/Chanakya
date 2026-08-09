# 🎨 UI Library Integration & Guide

This project is configured with **Kibo UI** and **Forge UI** along with **shadcn/ui** to build custom interactive interfaces.

---

## 🛠️ Step 1: Install Dependencies

Run the following commands to install required peer dependencies for animation, icons, and styling:

```bash
# In frontend/packages/ui
cd frontend/packages/ui
npm install framer-motion @radix-ui/react-slot lucide-react clsx tailwind-merge class-variance-authority tw-animate-css next-themes zod

# In frontend/apps/web
cd ../apps/web
npm install framer-motion lucide-react @tanstack/react-query @tanstack/react-table next-themes
```

---

## 📦 Step 2: Kibo UI Commands ([kibo-ui.com](https://www.kibo-ui.com/))

Navigate to `frontend/apps/web` and run any of these commands to add components:

```bash
# Add Kibo UI components using the CLI
npx kibo-ui add avatar-stack
npx kibo-ui add gantt
npx kibo-ui add kanban
npx kibo-ui add calendar
npx kibo-ui add dropzone
npx kibo-ui add code-block
npx kibo-ui add tags
npx kibo-ui add credit-card
npx kibo-ui add color-picker
npx kibo-ui add image-crop
npx kibo-ui add video-player
```

---

## 🚀 Step 3: Forge UI Commands ([forgeui.in](https://forgeui.in/))

Navigate to `frontend/apps/web` and run any of these commands to add components via shadcn CLI:

```bash
# Add Forge UI components
npx shadcn@latest add "https://forgeui.in/r/animated-form.json"
npx shadcn@latest add "https://forgeui.in/r/animated-tabs.json"
npx shadcn@latest add "https://forgeui.in/r/bot-detection.json"
npx shadcn@latest add "https://forgeui.in/r/fraud-card.json"
npx shadcn@latest add "https://forgeui.in/r/notification-center.json"
npx shadcn@latest add "https://forgeui.in/r/security-card.json"
npx shadcn@latest add "https://forgeui.in/r/text-morph.json"
npx shadcn@latest add "https://forgeui.in/r/text-shimmer.json"
npx shadcn@latest add "https://forgeui.in/r/expandable-card.json"
```

---

## 💻 Step 4: Run Development Server

```bash
cd frontend
npm run dev
```

Visit `http://localhost:3000/ui-demo` to see the live integration demo page.
