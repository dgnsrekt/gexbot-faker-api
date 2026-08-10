<script lang="ts">
  let { title, body }: { title: string; body: string } = $props()
</script>

<div class="wrap">
  <div class="card ph">
    <div class="ph-title">{title}</div>
    <div class="ph-body">{body}</div>
  </div>
</div>

<style>
  .wrap {
    padding: 18px 20px 28px;
  }
  .ph {
    padding: 22px;
    max-width: 620px;
  }
  .ph-title {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .ph-body {
    font-size: 12.5px;
    color: var(--muted);
    line-height: 1.6;
  }
</style>
