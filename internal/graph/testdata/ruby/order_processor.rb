class OrderProcessor
  def initialize(order)
    @order = order
  end

  def process
    validate_order
    calculate_total
    apply_discounts
    save_order
  end

  private

  def validate_order
    raise ArgumentError, "No items" if @order.items.empty?
  end

  def calculate_total
    @order.total = @order.items.sum(&:price)
  end

  def apply_discounts
    @order.total *= 0.9 if @order.total > 100
  end

  def save_order
    @order.save!
  end
end
