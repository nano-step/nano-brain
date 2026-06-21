class PaymentService
  def process(user)
    order = Order.new
    order.save
  end
end
